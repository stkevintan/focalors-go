package wechat

import (
	"focalors-go/client"
	"strings"
)

type BatchGetContactModel struct {
	UserNames    []string
	RoomWxIDList []string
}

type ChatRoomMember struct {
	UserName string `json:"user_name"`
	NickName string `json:"nick_name"`
	// 拉进群里的人
	Unknow string `json:"unknow"`
}

type ChatRoomData struct {
	MemberCount int              `json:"member_count"`
	MemberList  []ChatRoomMember `json:"chatroom_member_list"`
}
type ContactDetailModel struct {
	UserName        StrWrapper `json:"userName"`
	NickName        StrWrapper `json:"nickName"`
	BigHeadImgUrl   string     `json:"bigHeadImgUrl"`
	SmallHeadImgUrl string     `json:"smallHeadImgUrl"`
	HeadImgMd5      string     `json:"headImgMd5"`
}

type UserContactDetailModel struct {
	ContactDetailModel
	// user only
	Sex       int    `json:"sex"`
	Province  string `json:"province"`
	City      string `json:"city"`
	Signature string `json:"signature"` // 朋友圈签名
	Alias     string `json:"alias"`
}

type GetContactDetails[T ContactDetailModel | UserContactDetailModel | GroupContactDetailModel] struct {
	ContactCount int `json:"contactCount"`
	ContactList  []T `json:"contactList,omitempty"`
}

type GetContactDetailsApiResult[T ContactDetailModel | UserContactDetailModel | GroupContactDetailModel] struct {
	ApiResult
	Data GetContactDetails[T] `json:"Data"`
}

func (w *WechatClient) GetUserContactDetails(users ...string) (*GetContactDetailsApiResult[UserContactDetailModel], error) {
	res := &GetContactDetailsApiResult[UserContactDetailModel]{}
	if _, err := w.doPostAPICall("/friend/GetContactDetailsList", &BatchGetContactModel{
		UserNames: users,
		// RoomWxIDList: rooms,
	}, res); err != nil {
		return nil, err
	}
	return res, nil
}

type GetContactDetailsAll struct {
	Users []UserContactDetailModel
	Rooms []GroupContactDetailModel
}

func (w *WechatClient) GetGeneralContactDetails(ids ...string) (*GetContactDetailsAll, error) {
	users := []string{}
	rooms := []string{}
	for _, id := range ids {
		if strings.HasSuffix(id, "@chatroom") {
			rooms = append(rooms, id)
		} else {
			users = append(users, id)
		}
	}
	var res = &GetContactDetailsAll{}
	if len(users) > 0 {
		userRes, err := w.GetUserContactDetails(users...)
		if err != nil {
			return nil, err
		}
		res.Users = userRes.Data.ContactList
	}
	if len(rooms) > 0 {
		roomRes, err := w.GetChatRoomInfo(rooms...)
		if err != nil {
			return nil, err
		}
		res.Rooms = roomRes.Data.ContactList
	}
	return res, nil
}

func (c *UserContactDetailModel) AvatarUrl() string {
	return c.SmallHeadImgUrl
}

func (c *UserContactDetailModel) Nickname() string {
	return c.NickName.Str
}

func (c *UserContactDetailModel) Username() string {
	return c.UserName.Str
}

func (w *WechatClient) GetContactDetail(id ...string) ([]client.Contact, error) {
	details, err := w.GetGeneralContactDetails(id...)
	if err != nil {
		return nil, err
	}
	contacts := make([]client.Contact, 0, len(details.Users)+len(details.Rooms))
	for i := range details.Users {
		contacts = append(contacts, &details.Users[i])
	}
	for i := range details.Rooms {
		contacts = append(contacts, &details.Rooms[i])
	}
	return contacts, nil
}
