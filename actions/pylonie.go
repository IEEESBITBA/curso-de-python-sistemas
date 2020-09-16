package actions

import (
	"archive/zip"
	"crypto"
	"encoding/json"
	"fmt"
	"github.com/gobuffalo/envy"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v5"
	"github.com/gofrs/uuid"
	"go.etcd.io/bbolt"
)

const nullUUID = "00000000-0000-0000-0000-000000000000"

// recieve POST request to submit and evaluate code
func InterpretPost(c buffalo.Context) error {
	p := pythonHandler{}
	u := c.Value("current_user")
	if u == nil {
		return c.Render(403, r.HTML("index.plush.html"))
	}
	user := u.(*models.User)
	p.UserName = user.Name
	p.userID = encode([]rune(user.ID.String()), b64safe)
	if err := c.Bind(&p.code); err != nil {
		_ = p.codeResult(c, "", "An unexpected error occurred. You were logged out")
		return AuthDestroy(c)
	}
	if p.code.Evaluation.String() != nullUUID {
		return p.interpretEvaluation(c)
	}
	btx := c.Value("btx").(*bbolt.Tx)

	defer p.PutTx(btx, c)
	chrootPath := envy.Get("GONTAINER_FS", "")
	var err error
	if chrootPath == "" {
		err = p.runPy()
	} else {
		err = p.containerPy()
	}
	if err != nil {
		return p.codeResult(c, p.result.Output, err.Error())
	}
	return p.codeResult(c)
}

// This runs user submitted code and compares with evaluator output
// it then saves the result and writes to response.
// Interpreter will show if user submitted a correct result or incorrect
// or show line of error/exception
// List of prime numbers generator https://play.golang.org/p/ya7SuOH2-Ai
func (p pythonHandler) interpretEvaluation(c buffalo.Context) error {
	var ID big.Int
	ID.SetString(p.Input, 10)
	var lim big.Int
	lim.SetString("1000", 10)
	if p.Input == "" || len(p.Input) > 60 || ID.Cmp(&lim) == -1 || !ID.ProbablyPrime(6) {
		return p.codeResult(c, "", T.Translate(c, "curso-python-input-field-error"))
	}

	tx := c.Value("tx").(*pop.Connection)
	q := tx.Where("id = ?", p.code.Evaluation)
	exists, err := q.Exists("evaluations")
	if err != nil {
		return p.codeResult(c, "", T.Translate(c, "app-status-internal-error"))
	}
	if !exists {
		return p.codeResult(c, "", T.Translate(c, "curso-python-evaluation-not-found"))
	}
	eval := &models.Evaluation{}
	if err = q.First(eval); err != nil {
		return p.codeResult(c, "", T.Translate(c, "curso-python-evaluation-not-found"))
	}
	peval := pythonHandler{}
	peval.userID = p.userID
	peval.Source = eval.Solution
	peval.Input = p.Input //eval.Inputs.String
	chrootPath := envy.Get("GONTAINER_FS", "")
	if chrootPath == "" {
		err = peval.runPy()
	} else {
		err = peval.containerPy()
	}
	if err != nil {
		return p.codeResult(c, peval.Output, "Evaluation errored! "+err.Error()) // TODO this is the debug line
		//return  p.codeResult(c,"","Evaluation errored! "+err.Error()) // TODO this is the production line
	}
	btx := c.Value("btx").(*bbolt.Tx)
	defer p.PutTx(btx, c)
	p.Input = eval.Inputs.String
	err = p.runPy()
	if err != nil {
		return p.codeResult(c, p.Output, err.Error())
	}
	if p.Output == peval.Output {
		return p.codeResult(c, T.Translate(c, "curso-python-evaluation-success")+" ID:"+peval.Input)
	} else {
		return p.codeResult(c, "", T.Translate(c, "curso-python-evaluation-fail"))
	}
}

func ControlPanel(c buffalo.Context) error {
	return c.Render(200, r.HTML("curso/control-panel.plush.html"))
}

// Function to delete all python uploads
func DeletePythonUploads(c buffalo.Context) error {
	var auth struct {
		Key string `form:"authkey"`
	}
	if err := c.Bind(&auth); err != nil {
		return c.Error(500, err)
	}
	if auth.Key != ".b3060ee10d6305243c7b" {
		c.Flash().Add("warning", "bad key")
		return c.Redirect(302, "controlPanelPath()")
	}
	btx := c.Value("btx").(*bbolt.Tx)
	if err := btx.DeleteBucket([]byte(pyDBUploadBucketName)); err != nil {
		return c.Error(500, err)
	}
	c.Flash().Add("success", "Uploads deleted. Hope you know what you are doing")
	return c.Redirect(302, "/")
}

// adds code result to context response.
// First and second string inputs will replace
// stdout and stderr code output, respectively
// so be careful not to delete important output/error
// should be called only once per code upload
func (p *pythonHandler) codeResult(c buffalo.Context, output ...string) error {
	if len(output) == 1 {
		p.result.Output = output[0]
	}
	if len(output) == 2 {
		p.result.Output = output[0]
		p.result.Error = output[1]
	}
	if len(p.Output) > pyMaxOutputLength {
		p.Output = p.Output[:pyMaxOutputLength] + " \n" + T.Translate(c, "curso-python-interpreter-output-too-long")
	}
	jsonResponse, _ := json.Marshal(p.result)
	c.Response().WriteHeader(200) // all good status so tx is committed
	c.Response().Write(jsonResponse)
	return nil
}

// configuration values
const (
	pyCommand = "python3"

	pyTimeout_ms = 500
	// DB:
	// this Bucket name must coincide with one defined in init() in models/bbolt.go
	pyDBUploadBucketName = "pyUploads"
	pyMaxSourceLength    = 5000 // DB storage trim length
	pyMaxOutputLength    = 2000 // in characters
)

type pyExitStatus int

const (
	pyOK pyExitStatus = iota
	pyTimeout
	pyError
)

type code struct {
	Source     string    `json:"code" form:"code"`
	Input      string    `json:"input" form:"input"`
	Evaluation uuid.UUID `json:"evalid" form:"evalid"`
}

type result struct {
	Output  string        `json:"output"`
	Error   string        `json:"error"`
	Elapsed time.Duration `json:"elapsed"`
}

type pythonHandler struct {
	result
	code
	timecode time.Time
	Time     string `json:"time"`
	UserName string `json:"user"`
	userID   string
	filename string
}

// sanitization structures
var reForbid = map[*regexp.Regexp]string{
	regexp.MustCompile(`exec|eval|globals|locals|breakpoint|getattr|memoryview|vars|super`): "forbidden function key '%s'",
	//regexp.MustCompile(`input\s*\(`):                           "no %s) to parse!",
	regexp.MustCompile("tofile|savetxt|fromfile|fromtxt|load"): "forbidden numpy function key '%s'",
	regexp.MustCompile(`__\w+__`):                              "forbidden dunder function key '%s'",
}

// special treatment cursofor imports since we may allow special imports such as math, numpy, pandas
var reImport = regexp.MustCompile(`^from[\s]+[\w]+|import[\s]+[\w]+`)

var allowedImports = map[string]bool{
	"math":       true,
	"numpy":      true,
	"pandas":     true,
	"itertools":  false,
	"processing": false,
	"os":         false,
}

// This function runs python in a container (only works on linux)
// thus it is safe from hackers. Can't touch this requires installing
// github.com/soypat/gontainer in PATH. Also requires setting GONTAINER_FS
// to the path of the filesystem that will be containerized.
func (p *pythonHandler) containerPy() (err error) {
	chrootPath := envy.Get("GONTAINER_FS", "")
	if chrootPath == "" {
		return fmt.Errorf("GONTAINER_FS environment variable not set. see https://alpinelinux.org/ for a minimal filesystem")
	}
	err = p.code.sanitizePy()
	output := make([]byte, 0)
	if err != nil {
		return
	}
	userDir := fmt.Sprintf("/home/%s-%s", p.UserName, p.userID[0:5])
	if _, err := os.Stat(filepath.Join(chrootPath, userDir)); os.IsNotExist(err) {
		os.Mkdir(filepath.Join(chrootPath, userDir), os.ModeDir)
	}
	chrootFilename := filepath.Join(userDir, "f.py")
	filename := filepath.Join(chrootPath, chrootFilename)
	f, err := os.Create(filename)
	if err != nil {
		return
	}
	defer f.Close()

	_, err = f.Write([]byte(p.code.Source))
	if err != nil {
		return err
	}
	gontainerArgs := []string{"run", "--chdr", userDir, "--chrt", chrootPath, pyCommand, chrootFilename}
	cmd := exec.Command("gontainer", gontainerArgs...)
	stdin, _ := cmd.StdinPipe()
	go func() {
		stdin.Write([]byte(p.Input + "\n"))
	}()
	status := make(chan pyExitStatus, 1)
	go func() {
		time.Sleep(pyTimeout_ms * time.Millisecond)
		status <- pyTimeout
	}()
	var tstart time.Time
	go func() {
		tstart = time.Now()
		output, err = cmd.CombinedOutput()
		if err != nil {
			status <- pyError
		} else {
			status <- pyOK
		}
	}()
	select {
	case s := <-status:
		switch s {
		case pyTimeout:
			cmd.Process.Kill()
			return fmt.Errorf("process timed out (%dms)", pyTimeout_ms)
		case pyError, pyOK:
			p.Elapsed = time.Now().Sub(tstart)
			p.Output = strings.ReplaceAll(string(output), "\""+filename+"\",", "")
			return
		}
	}
	cmd.Process.Kill()
	return fmt.Errorf("server error.")

}

// this function runs python on the machine python
// installation. It creates a file in /tmp/{userID}
// and runs it as stdin. The combined output (stderr+stdout)
// is saved to the pythonHandler Output field.
func (p *pythonHandler) runPy() (err error) {
	err = p.code.sanitizePy()
	output := make([]byte, 0)
	if err != nil {
		return
	}
	os.Mkdir(fmt.Sprintf("tmp/%s", p.userID), os.ModeTemporary)
	if err != nil && err != os.ErrExist {
		return
	}
	filename := fmt.Sprintf("tmp/%s/f.py", p.userID)

	f, err := os.Create(filename)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write([]byte(p.code.Source))
	cmd := exec.Command(pyCommand, filename)
	stdin, _ := cmd.StdinPipe()
	go func() {
		stdin.Write([]byte(p.Input + "\n"))
	}()
	status := make(chan pyExitStatus, 1)
	go func() {
		time.Sleep(pyTimeout_ms * time.Millisecond)
		status <- pyTimeout
	}()
	var tstart time.Time
	go func() {
		tstart = time.Now()
		output, err = cmd.CombinedOutput()
		if err != nil {
			status <- pyError
		} else {
			status <- pyOK
		}
	}()
	for {
		select {
		case s := <-status:
			switch s {
			case pyTimeout:
				cmd.Process.Kill()
				return fmt.Errorf("process timed out (%dms)", pyTimeout_ms)
			case pyError, pyOK:
				p.Elapsed = time.Now().Sub(tstart)
				p.Output = strings.ReplaceAll(string(output), "\""+filename+"\",", "")
				return
			default:
				time.Sleep(time.Millisecond)
			}
		}
	}
}

func (c *code) sanitizePy() error {
	if len(c.Source) > 600 {
		return fmt.Errorf("code snippet too long!")
	}
	semicolonSplit := strings.Split(c.Source, ";")
	newLineSplit := strings.Split(c.Source, "\n")
	for _, v := range append(semicolonSplit, newLineSplit...) {
		for re, errF := range reForbid {
			str := re.FindString(strings.TrimSpace(v))
			if str != "" {
				return fmt.Errorf(errF, str)
			}
		}
		str := reImport.FindString(strings.TrimSpace(v))
		if str != "" {
			words := strings.Split(str, " ")
			if len(words) < 2 {
				return fmt.Errorf("unexpected import formatting: %s", str)
			}
			allowed, present := allowedImports[strings.TrimSpace(words[1])]
			if !present {
				return fmt.Errorf("import '%s' not in safelist:\n%s", strings.TrimSpace(words[1]), printSafeList())
			}
			if !allowed {
				return fmt.Errorf("forbidden import '%s'", strings.TrimSpace(words[1]))
			}
		}
	}
	return nil
}
func printSafeList() (s string) {
	counter := 0
	for k, v := range allowedImports {
		if v {
			counter++
			if counter > 1 {
				s += ",  "
			}
			s += k
		}
	}
	return
}

// Saves Python code and user to database
func (p *pythonHandler) PutTx(tx *bbolt.Tx, c buffalo.Context) {
	// closure eases error management
	err := func() error {
		b, err := tx.CreateBucketIfNotExists([]byte(pyDBUploadBucketName))
		if err != nil {
			return err
		}
		p.Time = time.Now().String()
		var pc pythonHandler
		pc = *p // because we don't want to store 5000000 length outputs
		if len(pc.Output) > pyMaxOutputLength {
			pc.Output = pc.Output[:pyMaxOutputLength]
		}
		if len(pc.Source) > pyMaxSourceLength {
			pc.Source = pc.Source[:pyMaxSourceLength]
		}
		buff, err := json.Marshal(pc)
		if err != nil {
			return err
		}
		h := crypto.MD5.New()
		h.Write([]byte(pc.UserName + pc.code.Source))
		sum := h.Sum(nil)
		if b.Get(sum) == nil {
			c.Logger().Infof("Code submitted user: %s", pc.UserName)
			return b.Put(h.Sum(nil), buff)
		}
		c.Logger().Infof("Repeated code submitted user: %s", pc.UserName)
		return nil
	}()

	if err != nil {
		c.Logger().Errorf("could not save python code to database for user '%s': %s\n", p.UserName, err.Error())
	}
}

// download contents of a bbolt.DB
// in raw database format. Programmed to be
// fast as heck
func boltDBDownload(db *bbolt.DB) func(c buffalo.Context) error {
	return func(c buffalo.Context) error {
		_, name := filepath.Split(db.Path())
		w := c.Response()
		err := db.View(func(tx *bbolt.Tx) error {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
			w.Header().Set("Content-Length", strconv.Itoa(int(tx.Size())))
			_, err := tx.WriteTo(w)
			return err
		})
		if err != nil {
			return c.Error(500, err)
		}
		return nil
	}
}

// Zips up a folder in relative path /assets and
// sends content to user requesting. Should be used sparingly
// for maintenance and admin tasks ideally.
func zipAssetFolder(path string) func(c buffalo.Context) error {
	return func(c buffalo.Context) error {
		jobname := encode([]rune(path), b64safe)
		fo, err := os.Create("tmp/" + jobname)
		if err != nil {
			c.Response().Write([]byte(err.Error()))
			return c.Redirect(500, "/")
		}
		w := c.Response()
		defer fo.Close()
		z := zip.NewWriter(fo)
		defer z.Close()
		fullpath := "assets/" + strings.Trim(path, "/\\") + "/"
		finfos, err := ioutil.ReadDir(fullpath)
		for _, f := range finfos {
			if f.IsDir() {
				continue
			}
			if err = addFileToZip(z, fullpath+f.Name()); err != nil {
				c.Response().Write([]byte(err.Error()))
				return c.Redirect(500, "/")
			}
		}
		z.Flush()
		if err := z.Close(); err != nil {
			c.Response().Write([]byte(err.Error()))
			return c.Redirect(500, "/")
		}

		info, _ := fo.Stat()
		fo.Close()
		//name := info.Name()

		// Add files to zip
		zipfile, err := os.Open("tmp/" + jobname)
		if err != nil {
			c.Response().Write([]byte(err.Error()))
			return c.Redirect(500, "/")
		}
		name := strings.Split(path, string(filepath.Separator))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name[len(name)-1]+`.zip"`)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		if _, err := io.Copy(w, zipfile); err != nil {
			return c.Redirect(500, "/")
		}
		return nil
	}
}

func addFileToZip(zipWriter *zip.Writer, filename string) error {

	fileToZip, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fileToZip.Close()

	// Get the file information
	info, err := fileToZip.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	// Using FileInfoHeader() above only uses the basename of the file. If we want
	// to preserve the folder structure we can overwrite this with the full path.
	//header.Name = filename // uncomment line to preserve folder structure

	// Change to deflate to gain better compression
	// see http://golang.org/pkg/archive/zip/#pkg-constants
	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, fileToZip)
	return err
}
