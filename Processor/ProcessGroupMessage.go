// 处理收到的信息事件
package Processor

import (
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/echo"
	"github.com/hoshinonyaruko/gensokyo/handlers"
	"github.com/hoshinonyaruko/gensokyo/idmap"
	"github.com/hoshinonyaruko/gensokyo/mylog"
	"github.com/hoshinonyaruko/gensokyo/unioncache"

	"github.com/tencent-connect/botgo/dto"
)

// ProcessGroupMessage 处理群组消息
func (p *Processors) ProcessGroupMessage(data *dto.WSGroupATMessageData, at_me bool) error {

	var userid64 int64
	var GroupID64 int64
	var err error

	if data.Author.ID == "" {
		mylog.Printf("出现ID为空未知错误.%v\n", data)
		return nil
	}

	// 改变之前先存
	if data.Author.UnionOpenID != "" && data.Author.ID != "" {
		unioncache.Store(data.Author.ID, data.Author.UnionOpenID)
	}

	// 全量群消息再mention中确定是否at me
	if !at_me {
		mylog.Printf("非at消息")
		at_me = checkMe(data)
	}
	if !config.GetStringOb11() {
		if config.GetIdmapPro() {
			//将真实id转为int userid64
			GroupID64, userid64, err = idmap.StoreIDv2Pro(data.GroupID, data.Author.ID)
			if err != nil {
				mylog.Errorf("Error storing ID: %v", err)
			}
			//当参数不全
			_, _ = idmap.StoreIDv2(data.GroupID)
			_, _ = idmap.StoreIDv2(data.Author.ID)
			if !config.GetHashIDValue() {
				mylog.Fatalf("避坑日志:你开启了高级id转换,请设置hash_id为true,并且删除idmaps并重启")
			}
			//补救措施
			idmap.SimplifiedStoreID(data.Author.ID)
			//补救措施
			idmap.SimplifiedStoreID(data.GroupID)
		} else {
			// 映射str的GroupID到int
			GroupID64, err = idmap.StoreIDv2(data.GroupID)
			if err != nil {
				mylog.Errorf("failed to convert GroupID64 to int: %v", err)
				return nil
			}
			// 映射str的userid到int
			userid64, err = idmap.StoreIDv2(data.Author.ID)
			if err != nil {
				mylog.Printf("Error storing ID: %v", err)
				return nil
			}
		}
	}

	var messageText string
	GetDisableErrorChan := config.GetDisableErrorChan()

	//当屏蔽错误通道时候=性能模式 不解析at 不解析图片
	if !GetDisableErrorChan {
		// 转换at
		messageText = handlers.RevertTransformedText(data, "group", p.Api, p.Apiv2, GroupID64, userid64, config.GetWhiteEnable(4))
		if messageText == "" {
			mylog.Printf("信息被自定义黑白名单拦截")
			return nil
		}

		//框架内指令
		p.HandleFrameworkCommand(messageText, data, "group")
	} else {
		// 减少无用的性能开支
		messageText = data.Content

		if messageText == "/ " {
			messageText = " "
		}

		if messageText == " / " {
			messageText = " "
		}
		messageText = strings.TrimSpace(messageText)

		// 检查是否需要移除前缀
		if config.GetRemovePrefixValue() {
			// 移除消息内容中第一次出现的 "/"
			if idx := strings.Index(messageText, "/"); idx != -1 {
				messageText = messageText[:idx] + messageText[idx+1:]
			}
		}

	}

	var messageID int
	//映射str的messageID到int
	if !config.GetStringOb11() {
		var messageID64 int64
		if config.GetMemoryMsgid() {
			messageID64, err = echo.StoreCacheInMemory(data.ID)
			if err != nil {
				log.Fatalf("Error storing ID: %v", err)
			}
		} else {
			messageID64, err = idmap.StoreCachev2(data.ID)
			if err != nil {
				log.Fatalf("Error storing ID: %v", err)
			}
		}
		messageID = int(messageID64)
	} else {
		messageID64, err := idmap.GenerateRowID(data.ID, 9)
		if err != nil {
			log.Fatalf("Error storing ID: %v", err)
		}
		messageID = int(messageID64)
	}

	// 如果在Array模式下, 则处理Message为Segment格式
	var segmentedMessages interface{} = messageText
	if config.GetArrayValue() {
		segmentedMessages = handlers.ConvertToSegmentedMessage(data)
	}

	var IsBindedUserId, IsBindedGroupId bool
	if !config.GetStringOb11() {
		if config.GetHashIDValue() {
			IsBindedUserId = idmap.CheckValue(data.Author.ID, userid64)
			IsBindedGroupId = idmap.CheckValue(data.GroupID, GroupID64)
		} else {
			IsBindedUserId = idmap.CheckValuev2(userid64)
			IsBindedGroupId = idmap.CheckValuev2(GroupID64)
		}
	}

	var selfid64 int64
	if config.GetUseUin() {
		selfid64 = config.GetUinint64()
	} else {
		selfid64 = int64(p.Settings.AppID)
	}

	//mylog.Printf("回调测试-群:%v\n", segmentedMessages)
	var groupMsg OnebotGroupMessage
	var groupMsgMap map[string]interface{}

	// 是否使用string形式上报
	if !config.GetStringOb11() {
		groupMsg = OnebotGroupMessage{
			RawMessage:    messageText,
			Message:       segmentedMessages,
			MessageID:     messageID,
			RealMessageID: data.ID,
			GroupID:       GroupID64,
			MessageType:   "group",
			PostType:      "message",
			SelfID:        selfid64,
			UserID:        userid64,
			Sender: Sender{
				UserID: userid64,
				Sex:    "0",
				Age:    0,
				Area:   "0",
				Level:  "0",
			},
			SubType: "normal",
			Time:    time.Now().Unix(),
			ToMe:    at_me,
		}
		//增强配置
		if !config.GetNativeOb11() {
			groupMsg.RealMessageType = "group"
			groupMsg.IsBindedUserId = IsBindedUserId
			groupMsg.IsBindedGroupId = IsBindedGroupId
			groupMsg.RealGroupID = data.GroupID
			groupMsg.RealUserID = data.Author.ID
			groupMsg.Avatar, _ = GenerateAvatarURLV2(data.Author.ID)
		}
		//根据条件判断是否增加nick和card
		var CaN = config.GetCardAndNick()
		if CaN != "" {
			groupMsg.Sender.Nickname = CaN
			groupMsg.Sender.Card = CaN
		}
		// 获取MasterID数组
		masterIDs := config.GetMasterID()

		// 判断userid64是否在masterIDs数组里
		isMaster := false
		for _, id := range masterIDs {
			if strconv.FormatInt(userid64, 10) == id {
				isMaster = true
				break
			}
		}

		// 根据isMaster的值为groupMsg的Sender赋值role字段
		if isMaster {
			groupMsg.Sender.Role = "owner"
		} else {
			groupMsg.Sender.Role = "member"
		}
		//储存当前群或频道号的类型
		idmap.WriteConfigv2(fmt.Sprint(GroupID64), "type", "group")
		//懒message_id池
		echo.AddLazyMessageId(strconv.FormatInt(GroupID64, 10), data.ID, time.Now())
		//懒message_id池
		echo.AddLazyMessageIdv2(strconv.FormatInt(GroupID64, 10), strconv.FormatInt(userid64, 10), data.ID, time.Now())
		// 如果要使用string参数action
		if config.GetStringAction() {
			//懒message_id池
			echo.AddLazyMessageId(data.GroupID, data.ID, time.Now())
			//懒message_id池
			echo.AddLazyMessageIdv2(data.GroupID, data.Author.ID, data.ID, time.Now())
		}
		// 调试
		PrintStructWithFieldNames(groupMsg)

		// Convert OnebotGroupMessage to map and send
		groupMsgMap = structToMap(groupMsg)
	} else {
		var imgurl string
		// 自用的地方,也有一点用,有图片的时候Sender.Area是图片url(这个字段本是废弃了)
		if len(data.Attachments) > 0 {
			imgurl = data.Attachments[0].URL
		}

		//将真实id转为int userid64
		userid64, err := idmap.GenerateRowID(data.Author.ID, 9)
		if err != nil {
			mylog.Errorf("Error storing ID: %v", err)
		}
		messageTime, err := data.Timestamp.Time()
		if err != nil {
			log.Fatalf("Error original timestamp: %v", err)
			messageTime = time.Now()
		}
		groupMsgS := OnebotGroupMessage{
			RawMessage:    messageText,
			Message:       segmentedMessages,
			MessageID:     messageID,
			RealMessageID: data.ID,
			GroupID:       GroupID64,
			MessageType:   "group",
			PostType:      "message",
			SelfID:        selfid64,
			UserID:        userid64,
			Sender: Sender{
				UserID: userid64,
				Sex:    "0",
				Age:    0,
				Area:   imgurl,
				Level:  "0",
			},
			SubType: "normal",
			Time:    messageTime.UnixMilli(),
			ToMe:    at_me,
		}
		// 增强配置
		if !config.GetNativeOb11() {
			groupMsgS.RealMessageType = "group"
			groupMsgS.RealGroupID = data.GroupID
			groupMsgS.RealUserID = data.Author.ID
			groupMsgS.Avatar, _ = GenerateAvatarURLV2(data.Author.ID)
		}
		//根据条件判断是否增加nick和card
		var CaN = config.GetCardAndNick()
		if CaN != "" {
			groupMsgS.Sender.Nickname = CaN
			groupMsgS.Sender.Card = CaN
		}
		// 获取MasterID数组
		masterIDs := config.GetMasterID()

		// 判断userid64是否在masterIDs数组里
		isMaster := slices.Contains(masterIDs, strconv.FormatInt(userid64, 10))
		// 根据isMaster的值为groupMsg的Sender赋值role字段
		if isMaster {
			groupMsgS.Sender.Role = "owner"
		} else {
			groupMsgS.Sender.Role = "member"
		}
		// 调试
		PrintStructWithFieldNames(groupMsgS)
		// Convert OnebotGroupMessage to map and send
		groupMsgMap = structToMap(groupMsgS)
	}

	// 如果不是性能模式
	if !GetDisableErrorChan {
		//上报信息到onebotv11应用端(正反ws) 并等待返回
		go p.BroadcastMessageToAll(groupMsgMap, p.Apiv2, data)
	} else {
		// FAF式
		go p.BroadcastMessageToAllFAF(groupMsgMap, p.Apiv2, data)
	}

	return nil
}

func checkMe(data *dto.WSGroupATMessageData) bool {
	mentions := data.Mentions
	if mentions == nil {
		return false
	}
	for _, mention := range mentions {
		if mention.IsYou == true {
			return true
		}
	}
	return false
}
