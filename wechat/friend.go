package wechat

type BatchGetContactModel struct {
	UserNames    []string
	RoomWxIDList []string
}

type ContactDetailModel struct {
	UserName        StrWrapper `json:"userName"`
	NickName        StrWrapper `json:"nickName"`
	Sex             int        `json:"sex"`
	BigHeadImgUrl   string     `json:"bigHeadImgUrl"`
	SmallHeadImgUrl string     `json:"smallHeadImgUrl"`
	HeadImgMd5      string     `json:"headImgMd5"`
}

type GetContactDetails struct {
	ContactCount int                  `json:"contactCount"`
	ContactList  []ContactDetailModel `json:"contactList"`
}

type GetContactDetailsApiResult struct {
	ApiResult
	Data GetContactDetails `json:"Data"`
}

func (w *WechatClient) GetContactDetails(users []string, rooms []string) (*GetContactDetailsApiResult, error) {
	res := &GetContactDetailsApiResult{}
	if _, err := w.doPostAPICall("/friend/GetContactDetailsList", &BatchGetContactModel{
		UserNames:    users,
		RoomWxIDList: rooms,
	}, res); err != nil {
		return nil, err
	}
	return res, nil
}

type GetChatRoomInfoModel struct {
	ChatRoomWxIdList []string `json:"ChatRoomWxIdList"`
}

// more room specific fields
func (w *WechatClient) GetChatRoomInfo(rooms []string) (*GetContactDetailsApiResult, error) {
	res := &GetContactDetailsApiResult{}
	if _, err := w.doPostAPICall("/group/GetChatRoomInfo", &GetChatRoomInfoModel{
		ChatRoomWxIdList: rooms,
	}, res); err != nil {
		return nil, err
	}
	return res, nil
}
