package wechat

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type MessageItem struct {
	ToUserName   string   // 接收者 wxid
	TextContent  string   // 文本类型消息时内容
	ImageContent string   // 图片类型消息时图片的 base64 编码
	MsgType      int      //1 Text 2 Image
	AtWxIDList   []string // 发送艾特消息时的 wxid 列表
}

type SendMessageModel struct {
	MsgItem []MessageItem // 消息体数组
}

func (w *WechatClient) SendTextMessage(messages []MessageItem) (*ApiResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages cannot be empty")
	}
	res := &ApiResult{}
	if _, err := w.doPostAPICall("/message/SendTextMessage", SendMessageModel{MsgItem: messages}, res); err != nil {
		return nil, err
	}
	return res, nil
}

func (w *WechatClient) SendImageMessage(messages []MessageItem) (*ApiResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages cannot be empty")
	}
	res := &ApiResult{}
	if _, err := w.doPostAPICall("/message/SendImageMessage", SendMessageModel{MsgItem: messages}, res); err != nil {
		return nil, err
	}
	return res, nil
}

func (w *WechatClient) SendImageNewMessage(messages []MessageItem) (*ApiResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages cannot be empty")
	}
	res := &ApiResult{}
	if _, err := w.doPostAPICall("/message/SendImageNewMessage", SendMessageModel{MsgItem: messages}, res); err != nil {
		return nil, err
	}
	return res, nil
}

type SendEmojiItem struct {
	ToUserName string
	EmojiMd5   string
	EmojiSize  int32
}

type SendEmojiMessageModel struct {
	EmojiList []SendEmojiItem
}

func (w *WechatClient) SendEmojiMessage(messages []SendEmojiItem) (*ApiResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages cannot be empty")
	}
	res := &ApiResult{}
	if _, err := w.doPostAPICall("/message/SendEmojiMessage", SendEmojiMessageModel{EmojiList: messages}, res); err != nil {
		return nil, err
	}
	return res, nil
}

type AppMessageItem struct {
	ToUserName  string
	ContentXML  string
	ContentType uint32 // 2001:(红包消息)
}

type AppMessageModel struct {
	AppList []AppMessageItem
}

func (w *WechatClient) SendAppMessage(messages []AppMessageItem) (*ApiResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages cannot be empty")
	}
	res := &ApiResult{}
	if _, err := w.doPostAPICall("/message/SendAppMessage", AppMessageModel{AppList: messages}, res); err != nil {
		return nil, err
	}
	return res, nil
}

type StrWrapper struct {
	Str string `json:"str"`
}

type WechatImageMessageBuf struct {
	Len    int    `json:"len"`
	Buffer string `json:"buffer"`
}

type MessageType int

const (
	TextMessage  MessageType = 1
	ImageMessage MessageType = 3
	EmojiMessage MessageType = 47
	AppMessage   MessageType = 49
)

type ChatType int

const (
	ChatTypePrivate ChatType = iota
	ChatTypeGroup   ChatType = iota
)

type WechatMessageBase struct {
	MsgId        int64                 `json:"msg_id"`
	MsgType      MessageType           `json:"msg_type"` // 1: 文本消息 3: 图片 47: emoji 49: app
	Status       int                   `json:"status"`
	ImgStatus    int                   `json:"img_status"`
	ImageBuf     WechatImageMessageBuf `json:"image_buf"`
	CreateTime   int64                 `json:"create_time"`
	MsgSource    string                `json:"msg_source"`
	PushContent  string                `json:"push_content"`
	NewMessageId int64                 `json:"new_message_id"`
}
type WechatSyncMessage struct {
	WechatMessageBase
	FromUserId StrWrapper `json:"from_user_name"`
	ToUserId   StrWrapper `json:"to_user_name"`
	Content    StrWrapper `json:"content"`
}

type WechatMessage struct {
	WechatMessageBase
	FromUserId  string   `json:"from_user_id"`
	ToUserId    string   `json:"to_user_id"`
	FromGroupId string   `json:"from_group_id"`
	ChatType    ChatType `json:"chat_type"`
	Content     string   `json:"content"`
}

/*{
    "msg_id": 504248632,
    "from_user_name": {
        "str": "wxid_xxxxx"
    },
    "to_user_name": {
        "str": "wxid_yyyyy"
    },
    "msg_type": 3,
    "content": {
        "str": "<?xml version=\"1.0\"?>\n<msg>\n\t<img aeskey=\"d4ac48bd0817673e39764f3af9913f8c\" encryver=\"1\" cdnthumbaeskey=\"d4ac48bd0817673e39764f3af9913f8c\" cdnthumburl=\"3057020100044b3049020100020449faf96f02032df7fa020462ebd476020468438f78042466653862633738342d336662322d346133332d396261612d643433653038373965313039020405290a020201000405004c505600\" cdnthumblength=\"5730\" cdnthumbheight=\"505\" cdnthumbwidth=\"505\" cdnmidheight=\"0\" cdnmidwidth=\"0\" cdnhdheight=\"0\" cdnhdwidth=\"0\" cdnmidimgurl=\"3057020100044b3049020100020449faf96f02032df7fa020462ebd476020468438f78042466653862633738342d336662322d346133332d396261612d643433653038373965313039020405290a020201000405004c505600\" length=\"476828\" md5=\"dc1ddf59ce1a154ddedc658c4eb1fbf8\" hevc_mid_size=\"40748\" originsourcemd5=\"20b4ba36e03eb33ef2ab960990926e77\">\n\t\t<secHashInfoBase64>eyJwaGFzaCI6IjMxYjAwMDQ0NDE4MDAwMDAiLCJwZHFIYXNoIjoiZmMwZGQ5ZTE4ZGJhZmYwOWYz\nODEwMzNlOWMzYTcxZDA0MTgyYTgyNGQ2ZGUyYjY2YTFhN2Y0OTM0YjRkYjMyNiJ9\n</secHashInfoBase64>\n\t\t<live>\n\t\t\t<duration>0</duration>\n\t\t\t<size>0</size>\n\t\t\t<md5 />\n\t\t\t<fileid />\n\t\t\t<hdsize>0</hdsize>\n\t\t\t<hdmd5 />\n\t\t\t<hdfileid />\n\t\t\t<stillimagetimems>0</stillimagetimems>\n\t\t</live>\n\t</img>\n\t<platform_signature />\n\t<imgdatahash />\n\t<ImgSourceInfo>\n\t\t<ImgSourceUrl />\n\t\t<BizType>0</BizType>\n\t</ImgSourceInfo>\n</msg>\n"
    },
    "status": 3,
    "img_status": 2,
    "img_buf": {
        "len": 5730,
        "buffer": "/9j/4AAQSkZ"
    },
    "create_time": 1749340629,
    "msg_source": "<msgsource>\n\t<sec_msg_node>\n\t\t<uuid>7d104b4de533360a58df27609cd14fea_</uuid>\n\t\t<risk-file-flag />\n\t\t<risk-file-md5-list />\n\t</sec_msg_node>\n\t<signature>N0_V1_2Sg42sgK|v1_caY/WODj</signature>\n\t<tmp_node>\n\t\t<publisher-id></publisher-id>\n\t</tmp_node>\n</msgsource>\n",
    "push_content": "Kevin : [图片]",
    "new_msg_id": 4267009255947656594
}
*/

/*
{
    "msg_id": 1734404168,
    "from_user_name": {
        "str": "wxid_xxxxx"
    },
    "to_user_name": {
        "str": "wxid_yyyyy"
    },
    "msg_type": 47,
    "content": {
        "str": "<msg><emoji fromusername = \"wxid_xxxxxx\" tousername = \"wxid_yyyyy\" type=\"2\" idbuffer=\"media:0_0\" md5=\"ddb728a76ac547b8651113f98e89c997\" len = \"6538845\" productid=\"\" androidmd5=\"ddb728a76ac547b8651113f98e89c997\" androidlen=\"6538845\" s60v3md5 = \"ddb728a76ac547b8651113f98e89c997\" s60v3len=\"6538845\" s60v5md5 = \"ddb728a76ac547b8651113f98e89c997\" s60v5len=\"6538845\" cdnurl = \"http://vweixinf.tc.qq.com/110/20402/stodownload?m=ddb728a76ac547b8651113f98e89c997&amp;filekey=30440201010430302e02016e0402535a04206464623732386137366163353437623836353131313366393865383963393937020363c65d040d00000004627466730000000132&amp;hy=SZ&amp;storeid=266f9f3fc0008a2591d40a02f0000006e01004fb2535a272908e0b6cb65369&amp;ef=1&amp;bizid=1022\" designerid = \"\" thumburl = \"\" encrypturl = \"http://vweixinf.tc.qq.com/110/20402/stodownload?m=85c6ce2f246fa0ab96d20b9cb8d82268&amp;filekey=30440201010430302e02016e0402535a04203835633663653266323436666130616239366432306239636238643832323638020363c660040d00000004627466730000000132&amp;hy=SZ&amp;storeid=266f9f3fc000e49611d40a02f0000006e02004fb2535a272908e0b6cb653f5&amp;ef=2&amp;bizid=1022\" aeskey= \"0cc39db3a7404019b3aaa6050b1a7221\" externurl = \"http://vweixinf.tc.qq.com/110/20403/stodownload?m=790814eb89696123084d2c1197134e1d&amp;filekey=30440201010430302e02016e0402535a04203739303831346562383936393631323303834643263313139373133346531640203035710040d00000004627466730000000132&amp;hy=SZ&amp;storeid=266f9f3fd00049de91d40a02f0000006e03004fb3535a272908e0b6cb65439&amp;ef=3&amp;bizid=1022\" externmd5 = \"10225d3455d689058f6b54c9f2e879f5\" width= \"300\" height= \"300\" tpurl= \"\" tpauthkey= \"\" attachedtext= \"\" attachedtextcolor= \"\" lensid= \"\" emojiattr= \"\" linkid= \"\" desc= \"\" ></emoji>  </msg>"
    },
    "status": 3,
    "img_status": 2,
    "img_buf": {
        "len": 0
    },
    "create_time": 1749340728,
    "msg_source": "<msgsource>\n\t<signature>N0_V1_pCCqqHg0|v1_iGbzEvDA</signature>\n\t<tmp_node>\n\t\t<publisher-id></publisher-id>\n\t</tmp_node>\n</msgsource>\n",
    "push_content": "Kevin : [动画表情]",
    "new_msg_id": 3877855390757711786
}
*/

func (w *WechatClient) SubscribeMessage(ctx context.Context, msgChan chan WechatMessage) error {
	url := fmt.Sprintf("%s?key=%s", w.cfg.SubURL, w.cfg.Token)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		logger.Error("Failed to connect to websocket", slog.Any("error", err))
		return err
	}
	for {
		select {
		case <-ctx.Done():
			logger.Info("Context canceled, stopping listening")
			return nil
		default:
			syncMessage := WechatSyncMessage{}
			err := conn.ReadJSON(&syncMessage)
			if err != nil {
				logger.Error("Failed to read message", slog.Any("error", err))
				if websocket.IsUnexpectedCloseError(err) {
					// websocket connection is closed
					return fmt.Errorf("websocket connection is closed: %w", err)
				}
				// sleep for a while before trying again
				time.Sleep(2 * time.Second)
				continue
			}
			message := WechatMessage{
				WechatMessageBase: syncMessage.WechatMessageBase,
				FromUserId:        syncMessage.FromUserId.Str,
				ToUserId:          syncMessage.ToUserId.Str,
				Content:           syncMessage.Content.Str,
			}

			if strings.HasSuffix(message.FromUserId, "@chatroom") {
				message.ChatType = ChatTypeGroup
			} else {
				message.ChatType = ChatTypePrivate
			}

			if message.ChatType == ChatTypeGroup {
				groupId := message.FromUserId
				splited := strings.SplitN(message.Content, ":\n", 2)
				if len(splited) == 2 {
					message.FromGroupId = groupId
					message.FromUserId = splited[0]
					message.Content = splited[1]
				} else {
					logger.Warn("Failed to split group message", slog.String("Content", message.Content))
				}
			}
			logger.Info("Received message",
				slog.String("FromUserId", message.FromUserId),
				slog.String("ToUserId", message.ToUserId),
				slog.String("MsgType", fmt.Sprintf("%d", message.MsgType)),
				slog.String("Content", message.PushContent),
			)

			msgChan <- message
		}
	}
}
