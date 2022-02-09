package cmd

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/ChenKS12138/remote-terminal/controller"
	"github.com/ChenKS12138/remote-terminal/dao"
	"github.com/ChenKS12138/remote-terminal/middleware"
	"github.com/gin-gonic/gin"
)

func loadConfig() {
	configUrl := flag.String("config", "", "load online configuration")
	proxyUrl := flag.String("proxy", "", "net proxy")
	bindAddr := flag.String("bind", "", "bind addr")
	flag.Parse()
	if len(*configUrl) == 0 || len(*bindAddr) == 0 {
		w := flag.CommandLine.Output()
		fmt.Fprintf(w, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(0)
	}

	log.Println("Config Ready To Init!")
	dao.InitConfig(*configUrl, *proxyUrl, *bindAddr)
	log.Println("Config Init Ready!")
}

func Boost() {
	loadConfig()
	configDao := dao.NewConfigDaoMust()

	index := controller.NewIndexController()
	container := controller.NewContainerController()
	oauth := controller.NewOauthController()
	static := controller.NewStaticController()

	r := gin.New()
	r.LoadHTMLGlob("template/*.html")

	r.Use(middleware.Recover(func(c *gin.Context, i interface{}) {
		log.Println(i)
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/oauth/redirect?error=%s&error_description=%s", "Unexpected Error", "service shutdown for unknown reason, try later!"))
		c.Abort()
	}))

	index.Group(r.Group("/"))
	container.Group(r.Group("/container"))
	oauth.Group(r.Group("/oauth"))
	static.Group(r.Group("/static"))

	srv := &http.Server{
		Addr:    configDao.BindAddr,
		Handler: r,
	}
	go func() {
		log.Println("Server Start")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutdown Server ...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown: %s\n", err)
	}
	log.Println("Server Exit")
}
