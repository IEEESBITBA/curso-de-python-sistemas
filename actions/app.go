package actions

import (
	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo-pop/v2/pop/popmw"
	"github.com/gobuffalo/envy"
	"github.com/gobuffalo/logger"
	csrf "github.com/gobuffalo/mw-csrf"
	forcessl "github.com/gobuffalo/mw-forcessl"
	i18n "github.com/gobuffalo/mw-i18n"
	paramlogger "github.com/gobuffalo/mw-paramlogger"
	"github.com/gobuffalo/packr/v2"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth/gothic"
	"github.com/unrolled/secure"
)

// ENV is used to help switch settings based on where the
// application is being run. Default is "development".
// GO_ENV = "production" for deployment
var ENV = envy.Get("GO_ENV", "development")
var app *buffalo.App

// T translator for context. use by calling T.Translate(ctx, "id")
var T *i18n.Translator

//var TT *i18n.
const hourDiffUTC = 3 // how many hours behind is UTC respect to current time. Argentina == 3h

// App is where all routes and middleware for buffalo
// should be defined. This is the nerve center of your
// application.
//
// Routing, middleware, groups, etc... are declared TOP -> DOWN.
// This means if you add a middleware to `app` *after* declaring a
// group, that group will NOT have that new middleware. The same
// is true of resource declarations as well.
//
// It also means that routes are checked in the order they are declared.
// `ServeFiles` is a CATCH-ALL route, so it should always be
// placed last in the route declarations, as it will prevent routes
// declared after it to never be called.
func App() *buffalo.App {
	if app == nil {
		app = buffalo.New(buffalo.Options{
			Host:         envy.Get("FORUM_HOST", envy.Get("HOST", "")),
			Env:          ENV,
			SessionName:  "_curso_session",
			LogLvl:       logger.InfoLevel,
			SessionStore: defaultCookieStore(),
		})
		// Automatically redirect to SSL. Only works if you have a proxy up and running such as NGINX
		// NGINX can be configured to do this for you so it's kind of useless. It also fucks up
		// google chrome's default redirection if you don't have the proxy on... dont use it
		//app.Use(forceSSL())

		// Log request parameters (filters apply).
		app.Use(paramlogger.ParameterLogger)

		// Protect against CSRF attacks. https://www.owasp.org/index.php/Cross-Site_Request_Forgery_(CSRF)
		// Remove to disable this.
		app.Use(csrf.New)

		// Wraps each request in a transaction.
		//  c.Value("tx").(*pop.Connection)
		// Remove to disable this.
		app.Use(popmw.Transaction(models.DB))
		app.Use(models.BBoltTransaction(models.BDB))
		// Setup and use translations:
		app.Use(translations())
		// -- Authorization/Security procedures --
		// sets user data in context from session data.
		app.Use(SetCurrentUser)
		app.Use(SafeList, Authorize) // AUTHORIZATION MIDDLEWARE. Checks if user is in safelist
		app.Use(SiteStruct)
		//app.GET("/auth", AuthHome)
		bah := buffalo.WrapHandlerFunc(gothic.BeginAuthHandler) // Begin authorization handler = bah
		auth := app.Group("/auth")
		auth.GET("/", AuthHome)
		auth.GET("/{provider}/callback", AuthCallback)
		auth.GET("/{provider}", bah)
		auth.DELETE("/logout", AuthDestroy).Name("logout")
		auth.Middleware.Skip(SafeList, bah, AuthCallback, AuthHome, AuthDestroy) // don't ask for user to be on safelist on authorization page
		auth.Middleware.Skip(Authorize, bah, AuthCallback, AuthHome)             // don't ask for authorization on authorization page
		auth.Middleware.Skip(SetCurrentUser, bah, AuthCallback, AuthHome)        // set current user needs to seek user in db. if no users present in db setcurrentuser fails

		searcher := app.Group("/s")
		searcher.GET("/", Search).Name("search")
		searcher.GET("/topic/{tid}", TopicSearch).Name("topicSearch")

		app.GET("/u/{uid}/unsubscribe/{tid}", UsersSettingsRemoveTopicSubscription).Name("topicUnsubscribe")
		app.GET("/u", UserSettingsGet).Name("userSettings")
		app.POST("/u", UserSettingsPost)
		app.GET("/favicon.ico", func(c buffalo.Context) error { // Browsers by default look for favicon at http://mysite.com/favico.ico
			return c.Redirect(301, "/assets/images/logo-curso32x32.png")
		})
		// home page setup
		app.GET("/", manageForum) //TODO change homepage
		app.Middleware.Skip(SafeList, manageForum)
		app.Middleware.Skip(Authorize, manageForum)

		// All things curso de python
		curso := app.Group("/curso-python")
		curso.GET("/eval", EvaluationIndex).Name("evaluation")
		curso.GET("/eval/create", CursoEvaluationCreateGet).Name("evaluationCreate")
		curso.POST("/eval/create", CursoEvaluationCreatePost)
		curso.GET("/eval/e/{evalid}", CursoEvaluationGet).Name("evaluationGet")
		curso.GET("/eval/e/{evalid}/edit", CursoEvaluationEditGet).Name("evaluationEditGet")
		curso.POST("/eval/e/{evalid}/edit", CursoEvaluationEditPost)
		curso.GET("/eval/e/{evalid}/delete", CursoEvaluationDelete).Name("evaluationDelete")

		interpreter := app.Group("/py")
		interpreter.POST("/", InterpretPost).Name("Interpret")

		// Actual forum stuiff
		forum := app.Group("/f/{forum_title}")
		forum.Use(SetCurrentForum)
		forum.GET("/edit", CreateEditForum).Name("forumEdit")
		forum.POST("/edit", EditForumPost)
		forum.GET("/", forumIndex).Name("forum")
		forum.GET("/create", CategoriesCreateGet).Name("catCreate")
		forum.POST("/create", CategoriesCreateOrEditPost)
		forum.Middleware.Skip(Authorize, forumIndex)
		forum.Middleware.Skip(SafeList, forumIndex)

		catGroup := forum.Group("/c/{cat_title}")
		catGroup.Use(SetCurrentCategory)
		catGroup.GET("/", CategoriesIndex).Name("cat")
		catGroup.GET("/createTopic", TopicCreateGet).Name("topicCreate")
		catGroup.POST("/createTopic", TopicCreatePost)
		catGroup.GET("/edit", CategoriesCreateGet).Name("catEdit")
		catGroup.POST("/edit", CategoriesCreateOrEditPost)
		catGroup.Middleware.Skip(Authorize, CategoriesIndex)
		catGroup.Middleware.Skip(SafeList, CategoriesIndex)

		topicGroup := catGroup.Group("/{tid}")

		topicGroup.Use(SetCurrentTopic)
		topicGroup.GET("/", TopicGet).Name("topicGet") //
		topicGroup.GET("/edit", TopicEditGet).Name("topicEdit")
		topicGroup.POST("/edit", TopicEditPost)
		topicGroup.GET("/delete", TopicDelete).Name("topicDelete")
		topicGroup.GET("/reply", ReplyGet).Name("reply")
		topicGroup.POST("/reply", ReplyPost)
		topicGroup.Middleware.Skip(Authorize, TopicGet)
		topicGroup.Middleware.Skip(SafeList, TopicGet)

		replyGroup := topicGroup.Group("/{rid}")
		replyGroup.Use(SetCurrentReply)
		replyGroup.GET("/edit", editReplyGet).Name("replyEdit")
		replyGroup.POST("/edit", editReplyPost)
		replyGroup.DELETE("/edit", DeleteReply)

		admin := app.Group("/admin")
		admin.Use(SiteStruct)
		admin.Use(AdminAuth, SafeList)
		admin.GET("/f", manageForum)
		admin.GET("newforum", CreateEditForum)
		admin.POST("newforum/post", createForumPost)

		admin.GET("users", UsersViewAllGet).Name("allUsers")
		admin.GET("users/{uid}/ban", BanUserGet).Name("banUser")
		admin.GET("users/{uid}/admin", AdminUserGet).Name("adminUser")
		admin.GET("users/{uid}/normalize", NormalizeUserGet).Name("normalizeUser")

		admin.GET("safelist", SafeListGet).Name("safeList")
		admin.POST("safelist", SafeListPost)
		admin.GET("/cbu", boltDBDownload(models.BDB)).Name("cursoCodeBackup")
		admin.GET("/cbureader", zipAssetFolder("uploadReader")).Name("cursoCodeBackupReader")
		admin.GET("/control-panel", ControlPanel).Name("controlPanel")
		admin.POST("/cbuDelete", DeletePythonUploads).Name("cursoCodeDelete")
		// We associate the HTTP 404 status to a specific handler.
		// All the other status code will still use the default handler provided by Buffalo.
		if ENV == "production" {
			//app.ErrorHandlers[404] = err404
			//app.ErrorHandlers[500] = err500
		}
		go runDBSearchIndex()
		app.ServeFiles("/", assetsBox) // serve files from the public directory

	}
	return app
}

// translations will load locale files, set up the translator `actions.T`,
// and will return a middleware to use to load the correct locale for each
// request.
// for more information: https://gobuffalo.io/en/docs/localization
func translations() buffalo.MiddlewareFunc {
	var err error
	if T, err = i18n.New(packr.New("app:locales", "../locales"), "es-ar"); err != nil {
		app.Stop(err)
	}
	return T.Middleware()
}

// forceSSL will return a middleware that will redirect an incoming request
// if it is not HTTPS. "http://example.com" => "https://example.com".
// This middleware does **not** enable SSL. for your application. To do that
// we recommend using a proxy: https://gobuffalo.io/en/docs/proxy
// for more information: https://github.com/unrolled/secure/
func forceSSL() buffalo.MiddlewareFunc {
	return forcessl.Middleware(secure.Options{
		SSLRedirect:     ENV == "production",
		SSLProxyHeaders: map[string]string{"X-Forwarded-Proto": "https"},
	})
}

// Call to must panics if err != nil
func must(err error) {
	if err != nil {
		panic(err)
	}
}

func err500(status int, err error, c buffalo.Context) error {
	u, ok := c.Value("current_user").(*models.User)
	if !ok || u == nil || u.Role != "admin" {
		return c.Render(500, r.HTML("meta/500.plush.html"))
	}
	c.Flash().Add("danger", "Internal server error (500): "+err.Error()) // T.Translate(c,"app-status-internal-error")
	return c.Render(500, r.HTML("meta/500.plush.html"))
}

func err404(status int, err error, c buffalo.Context) error {
	c.Flash().Add("warning", "Page not found (404)")    // T.Translate(c,"app-not-found")
	return c.Render(404, r.HTML("meta/404.plush.html")) //c.Redirect(302,"/")
}

func defaultCookieStore() sessions.Store {
	secret := envy.Get("SESSION_SECRET", "")
	if secret == "" && (ENV == "development" || ENV == "test") {
		secret = "buffalo-secret"
	}

	// In production a SESSION_SECRET must be set!
	if secret == "" {
		print("\nSESSION_SECRET not set in production\n")
	}

	cookieStore := sessions.NewCookieStore([]byte(secret))
	// SameSite field values: strict=3, Lax=2, None=4, Default=1. Need Lax for OAuth since we need external site cookie to authenticate!
	cookieStore.Options.SameSite = 2
	//Cookie secure attributes, see: https://www.owasp.org/index.php/Testing_for_cookies_attributes_(OTG-SESS-002)
	cookieStore.Options.HttpOnly = true
	if ENV == "production" {
		cookieStore.Options.Secure = true
	}
	return cookieStore
}
