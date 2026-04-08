package handlers

import (
	"fmt"
	"strconv"

	"github.com/hoshinonyaruko/gensokyo/callapi"
	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/echo"
	"github.com/hoshinonyaruko/gensokyo/idmap"
	"github.com/hoshinonyaruko/gensokyo/mylog"
	"github.com/tencent-connect/botgo/openapi"
)

func init() {
	callapi.RegisterHandler("send_msg", HandleSendMsg)
}

func HandleSendMsg(client callapi.Client, api openapi.OpenAPI, apiv2 openapi.OpenAPI, message callapi.ActionMessage) (string, error) {
	// 使用 message.Echo 作为key来获取消息类型
	var msgType string
	var retmsg string
	var err error
	if message.Params.MessageType != "" {
		msgType = message.Params.MessageType
	}
	if msgType == "" {
		if echoStr, ok := message.Echo.(string); ok {
			// 当 message.Echo 是字符串类型时执行此块
			msgType = echo.GetMsgTypeByKey(echoStr)
		}
	}
	// 检查GroupID是否为0
	checkZeroGroupID := func(id interface{}) bool {
		switch v := id.(type) {
		case int:
			return v != 0
		case int64:
			return v != 0
		case string:
			return v != "0" // 检查字符串形式的0
		default:
			return true // 如果不是int、int64或string，假定它不为0
		}
	}

	// 检查UserID是否为0
	checkZeroUserID := func(id interface{}) bool {
		switch v := id.(type) {
		case int:
			return v != 0
		case int64:
			return v != 0
		case string:
			return v != "0" // 同样检查字符串形式的0
		default:
			return true // 如果不是int、int64或string，假定它不为0
		}
	}

	if len(message.Params.GroupID.(string)) != 32 {
		if msgType == "" && message.Params.GroupID != nil && checkZeroGroupID(message.Params.GroupID) {
			msgType = GetMessageTypeByGroupid(config.GetAppIDStr(), message.Params.GroupID)
		}
		if msgType == "" && message.Params.UserID != nil && checkZeroUserID(message.Params.UserID) {
			msgType = GetMessageTypeByUserid(config.GetAppIDStr(), message.Params.UserID)
		}
		if msgType == "" && message.Params.GroupID != nil && checkZeroGroupID(message.Params.GroupID) {
			msgType = GetMessageTypeByGroupidV2(message.Params.GroupID)
		}
		if msgType == "" && message.Params.UserID != nil && checkZeroUserID(message.Params.UserID) {
			msgType = GetMessageTypeByUseridV2(message.Params.UserID)
		}
	}

	// New checks for UserID and GroupID being nil or 0
	if (message.Params.UserID == nil || !checkZeroUserID(message.Params.UserID)) &&
		(message.Params.GroupID == nil || !checkZeroGroupID(message.Params.GroupID)) {
		mylog.Printf("send_group_msgs接收到错误action: %v", message)
		return "", nil
	}

	switch msgType {
	case "group":
		//复用处理逻辑
		retmsg, _ = HandleSendGroupMsg(client, api, apiv2, message)
	case "guild":
		//用GroupID给ChannelID赋值,因为我们是把频道虚拟成了群
		message.Params.ChannelID = message.Params.GroupID.(string)
		var RChannelID string
		if message.Params.UserID != nil && config.GetIdmapPro() && message.Params.UserID.(string) != "" && message.Params.UserID.(string) != "0" {
			RChannelID, _, err = idmap.RetrieveRowByIDv2Pro(message.Params.ChannelID.(string), message.Params.UserID.(string))
			mylog.Printf("测试,通过Proid获取的RChannelID:%v", RChannelID)
		}
		if RChannelID == "" {
			// 使用RetrieveRowByIDv2还原真实的ChannelID
			RChannelID, err = idmap.RetrieveRowByIDv2(message.Params.ChannelID.(string))
		}
		if err != nil {
			mylog.Printf("error retrieving real RChannelID: %v", err)
		}
		message.Params.ChannelID = RChannelID
		retmsg, _ = HandleSendGuildChannelMsg(client, api, apiv2, message)
	case "guild_private":
		//send_msg比具体的send_xxx少一层,其包含的字段类型在虚拟化场景已经失去作用
		//根据userid绑定得到的具体真实事件类型,这里也有多种可能性
		//1,私聊(但虚拟成了群),这里用群号取得需要的id
		//2,频道私聊(但虚拟成了私聊)这里传递2个nil,用user_id去推测channel_id和guild_id
		retmsg, _ = HandleSendGuildChannelPrivateMsg(client, api, apiv2, message, nil, nil)
	case "group_private":
		//私聊信息
		retmsg, _ = HandleSendPrivateMsg(client, api, apiv2, message)
	case "forum":
		//用GroupID给ChannelID赋值,因为我们是把频道虚拟成了群
		message.Params.ChannelID = message.Params.GroupID.(string)
		var RChannelID string
		if message.Params.UserID != nil && config.GetIdmapPro() && message.Params.UserID.(string) != "" && message.Params.UserID.(string) != "0" {
			RChannelID, _, err = idmap.RetrieveRowByIDv2Pro(message.Params.ChannelID.(string), message.Params.UserID.(string))
			mylog.Printf("测试,通过Proid获取的RChannelID:%v", RChannelID)
		}
		if RChannelID == "" {
			// 使用RetrieveRowByIDv2还原真实的ChannelID
			RChannelID, err = idmap.RetrieveRowByIDv2(message.Params.ChannelID.(string))
		}
		if err != nil {
			mylog.Printf("error retrieving real RChannelID: %v", err)
		}
		message.Params.ChannelID = RChannelID
		retmsg, _ = HandleSendGuildChannelForum(client, api, apiv2, message)
	default:
		mylog.Printf("1Unknown message type: %s", msgType)
	}
	return retmsg, nil
}

// 通过user_id获取messageID
func GetMessageIDByUseridOrGroupid(appID string, userID interface{}) string {
	// 从appID和userID生成key
	var userIDStr string
	switch u := userID.(type) {
	case int:
		userIDStr = strconv.Itoa(u)
	case int64:
		userIDStr = strconv.FormatInt(u, 10)
	case float64:
		userIDStr = strconv.FormatFloat(u, 'f', 0, 64)
	case string:
		userIDStr = u
	default:
		// 可能需要处理其他类型或报错
		return ""
	}
	//将真实id转为int
	userid64, err := idmap.StoreIDv2(userIDStr)
	if err != nil {
		mylog.Printf("Error storing ID 241: %v", err)
		return ""
	}
	key := appID + "_" + fmt.Sprint(userid64)
	mylog.Printf("GetMessageIDByUseridOrGroupid_key:%v", key)
	messageid := echo.GetMsgIDByKey(key)
	if messageid == "" {
		key := appID + "_" + userIDStr
		mylog.Printf("GetMessageIDByUseridOrGroupid_key_2:%v", key)
		messageid = echo.GetMsgIDByKey(key)
	}
	return messageid
}

// 通过user_id获取messageID
func GetMessageIDByUseridOrGroupidSP(appID string, userID interface{}) string {
	// 从appID和userID生成key
	var userIDStr string
	switch u := userID.(type) {
	case int:
		userIDStr = strconv.Itoa(u)
	case int64:
		userIDStr = strconv.FormatInt(u, 10)
	case float64:
		userIDStr = strconv.FormatFloat(u, 'f', 0, 64)
	case string:
		userIDStr = u
	default:
		// 可能需要处理其他类型或报错
		return ""
	}

	messageid := echo.GetMsgIDv3(appID, userIDStr)
	return messageid
}

// 通过user_id获取EventID 私聊,群,频道,通用 userID可以是三者之一 这是不需要区分群+用户的 只需要精准到群 私聊只需要精准到用户 idmap不开启的用户使用
func GetEventIDByUseridOrGroupid(appID string, userID interface{}) string {
	// 从appID和userID生成key
	var userIDStr string
	switch u := userID.(type) {
	case int:
		userIDStr = strconv.Itoa(u)
	case int64:
		userIDStr = strconv.FormatInt(u, 10)
	case float64:
		userIDStr = strconv.FormatFloat(u, 'f', 0, 64)
	case string:
		userIDStr = u
	default:
		// 可能需要处理其他类型或报错
		return ""
	}
	//将真实id转为int 这是非idmap-pro的方式
	userid64, err := idmap.StoreIDv2(userIDStr)
	if err != nil {
		mylog.Printf("Error storing ID 241: %v", err)
		return ""
	}
	key := appID + "_" + fmt.Sprint(userid64)
	mylog.Printf("GetEventIDByUseridOrGroupid_key:%v", key)
	eventid := echo.GetEventIDByKey(key)
	if eventid == "" {
		// 用原始id获取,这个分支应该是没有用的.
		key := appID + "_" + userIDStr
		mylog.Printf("GetEventIDByUseridOrGroupid_key_2:%v", key)
		eventid = echo.GetEventIDByKey(key)
	}
	return eventid
}

// 通过user_id获取EventID 私聊,群,频道,通用 userID可以是三者之一 这是不需要区分群+用户的 只需要精准到群 私聊只需要精准到用户 idmap不开启的用户使用
func GetEventIDByUseridOrGroupidv2(appID string, userID interface{}) string {
	// 从appID和userID生成key
	var userIDStr string
	switch u := userID.(type) {
	case int:
		userIDStr = strconv.Itoa(u)
	case int64:
		userIDStr = strconv.FormatInt(u, 10)
	case float64:
		userIDStr = strconv.FormatFloat(u, 'f', 0, 64)
	case string:
		userIDStr = u
	default:
		// 可能需要处理其他类型或报错
		return ""
	}

	key := appID + "_" + userIDStr
	mylog.Printf("GetEventIDByUseridOrGroupid_key:%v", key)
	eventid := echo.GetEventIDByKey(key)

	return eventid
}

// 通过user_id获取messageID
func GetMessageIDByUseridAndGroupid(appID string, userID interface{}, groupID interface{}) string {
	// 从appID和userID生成key
	var userIDStr string
	switch u := userID.(type) {
	case int:
		userIDStr = strconv.Itoa(u)
	case int64:
		userIDStr = strconv.FormatInt(u, 10)
	case float64:
		userIDStr = strconv.FormatFloat(u, 'f', 0, 64)
	case string:
		userIDStr = u
	default:
		// 可能需要处理其他类型或报错
		return ""
	}
	// 从appID和userID生成key
	var GroupIDStr string
	switch u := groupID.(type) {
	case int:
		GroupIDStr = strconv.Itoa(u)
	case int64:
		GroupIDStr = strconv.FormatInt(u, 10)
	case float64:
		GroupIDStr = strconv.FormatFloat(u, 'f', 0, 64)
	case string:
		GroupIDStr = u
	default:
		// 可能需要处理其他类型或报错
		return ""
	}
	var userid64, groupid64 int64
	var err error
	if config.GetIdmapPro() {
		//将真实id转为int userid64
		groupid64, userid64, err = idmap.StoreIDv2Pro(GroupIDStr, userIDStr)
		if err != nil {
			mylog.Errorf("Error storing ID 210: %v", err)
		}
	} else {
		//将真实id转为int
		userid64, err = idmap.StoreIDv2(userIDStr)
		if err != nil {
			mylog.Errorf("Error storing ID 241: %v", err)
			return ""
		}
		//将真实id转为int
		groupid64, err = idmap.StoreIDv2(GroupIDStr)
		if err != nil {
			mylog.Errorf("Error storing ID 256: %v", err)
			return ""
		}
	}
	key := appID + "_" + fmt.Sprint(groupid64) + "_" + fmt.Sprint(userid64)
	mylog.Printf("GetMessageIDByUseridAndGroupid_key:%v", key)
	return echo.GetMsgIDByKey(key)
}
