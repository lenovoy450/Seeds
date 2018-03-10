package router

import (
	"github.com/gin-gonic/gin"
	"Seeds/utils"
	"net/http"
	"strconv"
	"time"
	"Seeds/models"
	"fmt"
)

type UserRouter struct {
	routerAble
}
var unableToProcessReq = gin.H{
	"ret": 0,
	"error": "Unable to process entity",
}

func getNode(context *gin.Context) (models.SsNode, bool) {
	db := utils.GetMySQLInstance()
	data, hasKey := context.GetQuery("node_id")

	if !hasKey {
		context.JSON(http.StatusUnprocessableEntity, unableToProcessReq)
		return models.SsNode{}, true
	}
	nodeId, err := strconv.ParseInt(data, 0, 64)

	if err != nil {
		context.JSON(http.StatusUnprocessableEntity, unableToProcessReq)
		return models.SsNode{}, true
	}

	var node models.SsNode
	err = db.Database.First(&node, "id = ?", nodeId).Error

	if err != nil {
		context.JSON(http.StatusNotFound, gin.H{
			"ret": 0,
			"message": "Node not found",
		})
		return models.SsNode{}, true
	}
	return node, false
}

func getUserList(context *gin.Context) {
	db := utils.GetMySQLInstance()

	node, notFound := getNode(context)

	if notFound {
		return
	}

	// Heartbeat
	node.NodeHeartbeat = time.Now().Unix()
	db.Database.Save(&node)

	if node.NodeBandwidthLimit != 0 {
		if node.NodeBandwidthLimit < node.NodeBandwidth {
			context.JSON(http.StatusOK, gin.H{
				"ret": 1,
				"data": []gin.H{},
			})
			return
		}
	}
	fmt.Println(time.Now().Format("2000-01-01 12:00:00"))
	var rawUsers []models.User
	query := db.Database.Where("class >= ?", node.NodeClass).
		Where("enable = ?", 1).Where("expire_in > ?", time.Now().Unix()).
		Or("is_admin = ?", 1)

	if node.NodeGroup != 0 {
		query = query.Where(models.User{NodeGroup: node.NodeGroup})
	}
	query.Find(&rawUsers)
	var users []gin.H

	for _, user := range rawUsers {
		users = append(users, gin.H{
			"method": user.Method,
			"obfs": user.Obfs,
			"obfs_param": user.ObfsParam,
			"protocol": user.Protocol,
			"protocol_param": user.ProtocolParam,
			"forbidden_ip": user.ForbiddenIp,
			"forbidden_port": user.ForbiddenPort,
			"node_speedlimit": user.NodeSpeedlimit,
			"disconnect_ip": user.DisconnectIp,
			"is_multi_user": user.IsMultiUser,
			"id": user.Id,
			"port": user.Port,
			"passwd": user.Passwd,
			"transfer_enable": user.U + user.D,
		})
	}

	context.JSON(http.StatusOK, gin.H{
		"ret": 1,
		"data": users,
	})
}

type TrafficData struct {
	U      int64 `json:"u"`
	D      int64 `json:"d"`
	UserId int64 `json:"user_id"`
}

type DataJSON struct {
	Data []TrafficData `json:"data"`
}

func addTraffic(context *gin.Context) {
	db := utils.GetMySQLInstance()
	node, notFound := getNode(context)
	if notFound {
		return
	}

	var body DataJSON
	context.BindJSON(&body)

	var totalBandwidth int64 = 0

	for _, data := range body.Data {
		var user models.User
		err := db.Database.First(&user, "id = ?", data.UserId).Error
		if err != nil {
			continue
		}
		user.T = int(time.Now().Unix())
		user.U += data.U
		user.D += data.D
		totalBandwidth += data.U + data.D
		db.Database.Save(&user)

		db.Database.Save(&models.UserTrafficLog{
			UserId: user.Id,
			U: data.U,
			D: data.D,
			NodeId: node.Id,
			Rate: node.TrafficRate,
			Traffic: strconv.Itoa(int(float64(data.U + data.D) * node.TrafficRate)),
			LogTime: int(time.Now().Unix()),
		})
	}
	node.NodeBandwidth += totalBandwidth
	db.Database.Save(&node)

	db.Database.Save(&models.SsNodeOnlineLog{
		NodeId:node.Id,
		OnlineUser: len(body.Data),
		LogTime: int(time.Now().Unix()),
	})

	context.JSON(http.StatusOK, gin.H{
		"ret": 1,
		"data": "ok",
	})
}

func (UserRouter) create(engine *gin.Engine) {
	userGroup := engine.Group("/users")
	{
		userGroup.GET("/", getUserList)
		userGroup.POST("/traffic", addTraffic)
	}
}