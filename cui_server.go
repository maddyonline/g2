package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/labstack/echo"
	mw "github.com/labstack/echo/middleware"
	"github.com/labstack/gommon/log"
	"github.com/maddyonline/goonj/cui"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

var opts struct {
	Port            string
	StaticFilesRoot string
}

func assignString(v *string, args ...string) {
	for _, arg := range args {
		if arg != "" {
			*v = arg
			break
		}
	}
}

const ENV_PORT_NAME = "CUI_PORT"
const ENV_STATIC_FILES_DIR = "CUI_STATIC_FILES_DIR"

const DEFAULT_PORT = "1323"
const DEFAULT_STATIC_DIR = "."

func initializeConfig() {
	flag.StringVar(&opts.Port, "port", "", "Port on which server runs")
	flag.StringVar(&opts.StaticFilesRoot, "static", "", "Path to static directory")
	flag.Parse()
	assignString(&opts.Port, opts.Port, os.Getenv(ENV_PORT_NAME), DEFAULT_PORT)
	defaultDir, err := filepath.Abs(DEFAULT_STATIC_DIR)
	if err != nil {
		log.Error("Got error while taking absolute path of %s: %v", DEFAULT_STATIC_DIR, err)
		defaultDir = "."
	}
	assignString(&opts.StaticFilesRoot, opts.StaticFilesRoot, os.Getenv(ENV_STATIC_FILES_DIR), defaultDir)
}

type (
	// Template provides HTML template rendering
	Template struct {
		templates *template.Template
	}
)

// Render HTML
func (t *Template) Render(w io.Writer, name string, data interface{}) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

// Handler
func hello(c *echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!\n")
}

func loadTemplates() *Template {
	t := &Template{
		// Cached templates
		templates: template.Must(template.ParseFiles("static_cui/cui/templates/cui.html")),
	}
	return t
}

var cui_html []byte

var tasks map[cui.TaskKey]*cui.Task
var toggle bool

func addCuiHandlers(e *echo.Echo) {
	c := e.Group("/c")
	c.Post("/_start", func(c *echo.Context) error {
		return c.String(http.StatusOK, "Started")
	})
	c.Post("/_get_task", func(c *echo.Context) error {
		val := &cui.ClientGetTaskMsg{
			Task:                 c.Form("task"),
			Ticket:               c.Form("ticket"),
			ProgLang:             c.Form("prg_lang"),
			HumanLang:            c.Form("human_lang"),
			PreferServerProgLang: c.Form("prefer_server_prg_lang") == "false",
		}
		return c.XML(http.StatusOK, cui.GetTask(tasks, val))
	})

	chk := e.Group("/chk")
	chk.Post("/clock", func(c *echo.Context) error {
		return c.XML(http.StatusOK, cui.GetClock())
	})
	chk.Post("/save", func(c *echo.Context) error {
		val := struct {
			Task     string
			Ticket   string
			ProgLang string
			Solution string
		}{
			Task:     c.Form("task"),
			Ticket:   c.Form("ticket"),
			ProgLang: c.Form("prg_lang"),
			Solution: c.Form("solution"),
		}
		v, _ := json.Marshal(val)
		log.Info("%s", string(v))
		key := cui.TaskKey{val.Ticket, val.Task}
		tasks[key].CurrentSolution = val.Solution
		tasks[key].ProgLang = val.ProgLang
		return c.String(http.StatusOK, "Finished saving")
	})

	chk.Post("/verify", func(c *echo.Context) error {
		toggle = !toggle
		return c.XML(http.StatusOK, cui.GetVerifyStatus(toggle))
	})
	chk.Post("/final", func(c *echo.Context) error {
		return c.XML(http.StatusOK, cui.GetVerifyStatus(true))
	})
}

func main() {
	initializeConfig()
	port := opts.Port
	staticDir := filepath.Join(opts.StaticFilesRoot, "static_cui/cui/static/cui")
	log.Info("Using PORT=%s", port)
	log.Info("Using StaticDir=%s", staticDir)

	tasks = map[cui.TaskKey]*cui.Task{}
	// Echo instance
	e := echo.New()
	e.Hook(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		l := len(path) - 1
		if path != "/" && path[l] == '/' {
			r.URL.Path = path[:l]
		}
	})
	t := loadTemplates()
	e.SetRenderer(t)

	// Middleware
	e.Use(mw.Logger())
	e.Use(mw.Recover())

	// Routes
	e.Get("/hello", hello)
	e.Static("/static/cui", staticDir)
	e.Get("/cui", func(c *echo.Context) error {
		type Ticket struct {
			Id string
		}
		ticket := &Ticket{RandId()}
		return c.Render(http.StatusOK, "cui.html", map[string]interface{}{"Title": "Goonj", "Ticket": ticket})
	})

	addCuiHandlers(e)

	// Start server
	e.Run(fmt.Sprintf(":%s", port))
}
