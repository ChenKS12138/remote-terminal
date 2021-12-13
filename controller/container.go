package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/ChenKS12138/remote-terminal/auth"
	"github.com/ChenKS12138/remote-terminal/auth/permission"
	dao "github.com/ChenKS12138/remote-terminal/dao"
	"github.com/ChenKS12138/remote-terminal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upGrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WsMessage struct {
	Type int         `json:"type"`
	Data interface{} `json:"data"`
}

type WsTerminalIOData struct {
	c *chan []byte
}

func (w *WsTerminalIOData) Read(p []byte) (int, error) {
	data, ok := <-*(w.c)
	copy(p, data)
	if !ok {
		return 0, errors.New("data chan fail")
	}
	return len(data), nil
}

type WsTerminalIO struct {
	conn       *websocket.Conn
	dataChan   chan []byte
	resizeChan chan [2]float64
	IOData     *WsTerminalIOData
}

func NewWsTerminalIO(conn *websocket.Conn) *WsTerminalIO {
	dataChan := make(chan []byte)
	resizeChan := make(chan [2]float64)
	return &WsTerminalIO{
		conn:       conn,
		dataChan:   dataChan,
		resizeChan: resizeChan,
		IOData: &WsTerminalIOData{
			c: &dataChan,
		},
	}
}

func (w *WsTerminalIO) Boost() error {
	for {
		_, data, err := w.conn.ReadMessage()
		if err != nil {
			return err
		}
		msg := &WsMessage{}
		json.Unmarshal(data, msg)
		switch msg.Type {
		case 0:
			w.dataChan <- []byte(msg.Data.(string))
		case 1:
			size := msg.Data.([]interface{})
			w.resizeChan <- [2]float64{size[0].(float64), size[1].(float64)}
		default:
			return errors.New("unexpected message type")
		}
	}
}

func (w *WsTerminalIO) Write(p []byte) (n int, err error) {
	err = w.conn.WriteMessage(websocket.BinaryMessage, p)
	return len(p), err
}

type ContainerController struct{}

func NewContainerController() *ContainerController {
	return &ContainerController{}
}

func (cc *ContainerController) connect(c *gin.Context) {
	claim := auth.GetClaim(c)

	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer ws.Close()
	wsCloseChan := make(chan interface{})
	ws.SetCloseHandler(func(_ int, _ string) error {
		wsCloseChan <- nil
		return nil
	})

	wsTerminalIO := NewWsTerminalIO(ws)
	go wsTerminalIO.Boost()
	wsTerminalIO.Write([]byte(fmt.Sprintf("%s, Welcome To Use Remote Terminal!\r\n", claim.GithubLogin)))

	containerDao, err := dao.NewContainerDao()
	if err != nil {
		panic(err)
	}
	container, err := containerDao.FindByID(claim.GithubLogin)
	if err != nil {
		panic(err)
	}

	if container == nil {
		wsTerminalIO.Write([]byte("Container Not Found!\r\nAllocating...\r\n"))
		container, err = containerDao.CreateByID(claim.GithubLogin, wsTerminalIO)
		if err != nil {
			panic(err)
		}
	} else {
		wsTerminalIO.Write([]byte(fmt.Sprintf("Container Found!\r\nImage: %s\r\nReusing...\r\n", container.Image)))
	}
	wsTerminalIO.Write([]byte("Container Ready!\r\n"))
	// defer func() {
	// 	go containerDao.Shutdown(container)
	// }()
	if err := containerDao.AttachAndWait(container, wsTerminalIO.IOData, wsTerminalIO, wsCloseChan, wsTerminalIO.resizeChan); err != nil {
		log.Println(err)
		panic(err)
	}
}

func (c *ContainerController) Group(g *gin.RouterGroup) {
	g.Use(middleware.Recover(func(c *gin.Context, i interface{}) {
		err, errOk := i.(error)
		if errOk && err == auth.ErrAuthorizationFail {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    -1,
				"message": "unauthorized",
			})
		}
	}), auth.RequireAuth(permission.RunOwnContainer))
	g.GET("/connect", c.connect)
}
