package controller

import (
	"fmt"
	"mime"
	"net/http"
	"path"

	dao "github.com/ChenKS12138/remote-terminal/dao"
	"github.com/gin-gonic/gin"
)

type StaticController struct{}

func NewStaticController() *StaticController {
	return &StaticController{}
}

func index(c *gin.Context) {
	staticDao := dao.NewStaticDao()
	filepath := c.Params.ByName("filename")
	data, err := staticDao.Get(path.Join("static", filepath))
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/oauth/redirect?error=%s&error_description=%s", "Resource Error", err.Error()))
	}
	c.Data(http.StatusOK, mime.TypeByExtension(path.Ext(filepath)), data)
}


func (sc *StaticController) Group(g *gin.RouterGroup) {
	g.GET("/*filename", index)
}
