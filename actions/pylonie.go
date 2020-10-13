package actions

import (
	"archive/zip"
	"crypto"
	"encoding/json"
	"fmt"
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

	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/mailers"
	"github.com/gobuffalo/envy"

	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v5"
	"github.com/gofrs/uuid"
	"go.etcd.io/bbolt"
)

const nullUUID = "00000000-0000-0000-0000-000000000000"

// InterpretPost receive POST request to submit and evaluate code
func InterpretPost(c buffalo.Context) error {
	p := pythonHandler{}
	u := c.Value("current_user")
	if u == nil {
		return c.Render(403, r.HTML("index.plush.html"))
	}
	user := u.(*models.User)
	p.UserName = user.Name
	p.userID = Encode([]rune(user.ID.String()), Abc64safe)
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

// interpretEvaluation This runs user submitted code and compares with evaluator output
// it then saves the result and writes to response.
// Interpreter will show if user submitted a correct result or incorrect
// or show line of error/exception
// List of prime numbers generator https://play.golang.org/p/ya7SuOH2-Ai
func (p pythonHandler) interpretEvaluation(c buffalo.Context) error {
	// The value obtained from code submission as `input` is the team ID in the context of an
	// evaluation. Reason being that there is no other input a user can have for the time being.
	user := c.Value("current_user").(*models.User)
	btx := c.Value("btx").(*bbolt.Tx)
	if p.Exists(btx, c) && user.Subscribed(p.code.Evaluation) { // if code is duplicate and user already passed error out
		return p.codeResult(c, "", T.Translate(c, "curso-python-evaluation-duplicate"))
	}

	teamID := p.Input
	var ID big.Int
	ID.SetString(teamID, 10)
	var lim big.Int
	lim.SetString("2000", 10)
	if p.Input == "" || len(teamID) > 60 || ID.Cmp(&lim) == -1 || !ID.ProbablyPrime(6) {
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
	chrootPath := envy.Get("GONTAINER_FS", "")
	tests := strings.Split(strings.ReplaceAll(eval.Inputs.String, "\r", ""), "---\n")
	passed := 0

	for _, test := range tests {
		peval.Input = teamID + "\n" + test
		p.Input = test

		if err = peval.runPy(); err != nil {
			if c.Value("role").(string) == "admin" {
				return p.codeResult(c, peval.Output, "Evaluation errored! "+err.Error())
			}
			return p.codeResult(c, "", "Evaluation errored! "+err.Error())
		}
		if chrootPath == "" {
			err = p.runPy()
		} else {
			err = p.containerPy()
		}

		if err != nil {
			return p.codeResult(c, p.Output, err.Error())
		}
		if p.Output == peval.Output {
			passed++
		} else {
			p.Elapsed[len(p.Elapsed)-1] = 0
		}
	}
	defer p.PutTx(btx, c)
	if float64(passed)/float64(len(tests)) < 0.4 {
		msg := fmt.Sprintf("%s ID:%s\n(%d/%d) casos bien", T.Translate(c, "curso-python-evaluation-fail"), teamID, passed, len(tests))
		return p.codeResult(c, "", msg)
	}
	user.AddSubscription(eval.ID)
	_ = tx.UpdateColumns(user, "subscriptions")
	msg := fmt.Sprintf("%s ID:%s\n(%d/%d) casos bien", T.Translate(c, "curso-python-evaluation-success"), teamID, passed, len(tests))
	err = newEvaluationSuccessNotify(c, eval) // this is the same as go newEvaluationSuccessNotify(c,eval). The closure is to avoid golint from picking up errors
	if err != nil {
		c.Logger().Errorf("sending evaluation success mail to %s", user.Email)
	}
	return p.codeResult(c, msg)
}

// DeletePythonUploads delete all python uploads in bbolt DB
func DeletePythonUploads(c buffalo.Context) error {
	btx := c.Value("btx").(*bbolt.Tx)
	if err := btx.DeleteBucket([]byte(pyDBUploadBucketName)); err != nil {
		return c.Error(500, err)
	}
	c.Flash().Add("success", "Uploads deleted. Hope you know what you are doing")
	return c.Redirect(302, "/")
}

// codeResult adds code result to context response.
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
	_, _ = c.Response().Write(jsonResponse)
	return nil
}

// configuration values
const (
	pyCommand = "python3"
	// [Milliseconds] after running python code for this time the process is killed
	pyTimeoutMS = 500
	// DB:
	// this Bucket name must coincide with one defined in init() in models/bbolt.go
	pyDBUploadBucketName = "pyUploads"
	pyMaxSourceLength    = 1200 // DB storage trim length
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
	Output  string          `json:"output"`
	Error   string          `json:"error"`
	Elapsed []time.Duration `json:"elapsed"`
}

type pythonHandler struct {
	result
	code
	Timecode time.Time
	Time     string `json:"time"`
	UserName string `json:"user"`
	userID   string
	Filename string `json:"-" form:"-"`
}

// reForbid sanitization structures
var reForbid = map[*regexp.Regexp]string{
	regexp.MustCompile(`exec|eval|globals|locals|write|breakpoint|getattr|memoryview|vars|super`): "forbidden function key '%s'",
	//regexp.MustCompile(`input\s*\(`):                           "no %s) to parse!",
	regexp.MustCompile("tofile|savetxt|fromfile|fromtxt|load"):                                                                                  "forbidden numpy function key '%s'",
	regexp.MustCompile("to_csv|to_json|to_html|to_clipboard|to_excel|to_hdf|to_feather|to_parquet|to_msgpack|to_stata|to_pickle|to_sql|to_gbq"): "forbidden pandas function key '%s'",
	regexp.MustCompile(`__\w+__`): "forbidden dunder function key '%s'",
}

// special treatment for imports since we may allow special imports such as math, numpy, pandas
var reImport = regexp.MustCompile(`^from[\s]+[\w]+|import[\s]+[\w]+`)

var allowedImports = map[string]bool{
	"math":       true,
	"numpy":      true,
	"pandas":     true,
	"itertools":  false,
	"processing": false,
	"os":         false,
}

const maxInt64 = 1<<63 - 1

// containerPy This function runs python in a container (only works on linux)
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
		_ = os.Mkdir(filepath.Join(chrootPath, userDir), os.ModeDir)
	}
	chrootFilename := filepath.Join(userDir, "f.py")
	filename := filepath.Join(chrootPath, chrootFilename)
	f, err := os.Create(filename)
	if err != nil {
		return
	}
	defer func() {
		f.Close()
	}()
	_, err = f.Write([]byte(p.code.Source))
	if err != nil {
		return err
	}
	timeoutDuration := time.Millisecond * pyTimeoutMS
	gontainerArgs := []string{"run", "--chdr", userDir, "--chrt", chrootPath,
		"--timeout", (timeoutDuration + time.Second).String(), pyCommand, chrootFilename}
	cmd := exec.Command("gontainer", gontainerArgs...)
	defer func() {
		cmd.Process.Kill()
	}()
	stdin, _ := cmd.StdinPipe()
	go func() {
		_, _ = stdin.Write([]byte(p.Input + "\n"))
	}()
	status := make(chan pyExitStatus, 1)
	go func() {
		time.Sleep(timeoutDuration)
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

	switch <-status {
	case pyTimeout:
		p.Elapsed = append(p.Elapsed, timeoutDuration)
		return fmt.Errorf("process timed out (%dms)", pyTimeoutMS)
	case pyError, pyOK:
		p.Elapsed = append(p.Elapsed, time.Since(tstart))
		replaced := strings.ReplaceAll(string(output), "\""+chrootFilename+"\",", "")
		if strings.Index(string(output), chrootFilename) > 0 {
			return fmt.Errorf(replaced)
		}
		p.Output = replaced
		return
	}
	return fmt.Errorf("server error in container")

}

// runPy this function runs python on the machine python
// installation. It creates a file in /tmp/{userID}
// and runs it as stdin. The combined output (stderr+stdout)
// is saved to the pythonHandler Output field.
func (p *pythonHandler) runPy() (err error) {
	err = p.code.sanitizePy()
	output := make([]byte, 0)
	if err != nil {
		return
	}
	err = os.Mkdir(fmt.Sprintf("tmp/%s", p.userID), os.ModeDir)
	if err != nil && os.IsNotExist(err) {
		return fmt.Errorf("in runPy() mkdir: %s", err)
	}
	filename := fmt.Sprintf("tmp/%s/f.py", p.userID)
	f, err := os.Create(filename)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write([]byte(p.code.Source))
	cmd := exec.Command(pyCommand, filename)
	stdin, _ := cmd.StdinPipe()
	go func() {
		_, _ = stdin.Write([]byte(p.Input + "\n"))
	}()
	status := make(chan pyExitStatus, 1)
	go func() {
		time.Sleep(pyTimeoutMS * time.Millisecond)
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

	switch stat := <-status; stat {
	case pyTimeout:
		_ = cmd.Process.Kill()
		p.Elapsed = append(p.Elapsed, time.Millisecond*pyTimeoutMS)
		return fmt.Errorf("process timed out (%dms)", pyTimeoutMS)
	case pyError, pyOK:
		p.Elapsed = append(p.Elapsed, time.Since(tstart))
		p.Output = strings.ReplaceAll(string(output), "\""+filename+"\",", "")
		return
	}
	_ = cmd.Process.Kill()
	return nil
}

func (c *code) sanitizePy() error {
	if len(c.Source) > pyMaxSourceLength {
		return fmt.Errorf("code snippet too long (%d/%d)", len(c.Source), pyMaxSourceLength)
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

// printSafeList shows user what imports can
// be used in interpreter
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

// Exists check if code has already been submitted to database
// Depends on pythonhandler having both Source and UserName fields
func (p *pythonHandler) Exists(tx *bbolt.Tx, c buffalo.Context) bool {
	src := p.Source
	if len(src) > pyMaxSourceLength {
		src = src[:pyMaxSourceLength]
	}
	b := tx.Bucket([]byte(pyDBUploadBucketName))
	h := crypto.MD5.New()
	_, _ = h.Write([]byte(p.UserName + src))
	sum := h.Sum(nil)
	return b.Get(sum) != nil
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
		// var pc pythonHandler
		pc := *p // because we don't want to store 5000000 length outputs
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
		_, _ = h.Write([]byte(pc.UserName + pc.code.Source))
		sum := h.Sum(nil)
		if b.Get(sum) == nil {
			c.Logger().Infof("Code submitted user: %s", pc.UserName)
			return b.Put(sum, buff)
		}
		c.Logger().Infof("Repeated code submitted user: %s", pc.UserName)
		return nil
	}()

	if err != nil {
		c.Logger().Errorf("could not save python code to database for user '%s': %s\n", p.UserName, err)
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

// newEvaluationSuccessNotify logs error status to context
func newEvaluationSuccessNotify(c buffalo.Context, eval *models.Evaluation) error {
	var recpts []models.User
	user := c.Value("current_user").(*models.User)
	recpts = append(recpts, *user)
	// mailer checks if user already passed evaluation
	err := mailers.NewEvaluationSuccessNotify(c, eval, recpts)
	if err != nil {
		c.Logger().Errorf("evaluation: fail to send %s success mail", user.Name)
	} else {
		c.Logger().Infof("evaluation: success sending pass mail to %s", user.Name)
	}
	return nil
}

// zipAssetFolder Zips up a folder in relative path /assets and
// sends content to user requesting. Should be used sparingly
// for maintenance and admin tasks ideally.
func zipAssetFolder(path string) func(c buffalo.Context) error {
	return func(c buffalo.Context) error {
		jobname := Encode([]rune(path), Abc64safe)
		fo, err := os.Create("tmp/" + jobname)
		if err != nil {
			_, _ = c.Response().Write([]byte(err.Error()))
			return c.Redirect(500, "/")
		}
		w := c.Response()
		defer fo.Close()
		z := zip.NewWriter(fo)
		defer z.Close()
		fullpath := "assets/" + strings.Trim(path, "/\\") + "/"
		finfos, err := ioutil.ReadDir(fullpath)
		if err != nil {
			return c.Error(500, err)
		}
		for _, f := range finfos {
			if f.IsDir() {
				continue
			}
			if err = addFileToZip(z, fullpath+f.Name()); err != nil {
				_, _ = c.Response().Write([]byte(err.Error()))
				return c.Redirect(500, "/")
			}
		}
		z.Flush()
		if err := z.Close(); err != nil {
			_, _ = c.Response().Write([]byte(err.Error()))
			return c.Redirect(500, "/")
		}

		info, _ := fo.Stat()
		fo.Close()
		//name := info.Name()

		// Add files to zip
		zipfile, err := os.Open("tmp/" + jobname)
		if err != nil {
			_, _ = c.Response().Write([]byte(err.Error()))
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
