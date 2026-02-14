package wechat

type GetChatRoomInfoModel struct {
	ChatRoomWxIdList []string `json:"ChatRoomWxIdList"`
}

type GroupContactDetailModel struct {
	ContactDetailModel
	NewChatRoomData ChatRoomData `json:"newChatRoomData"`
}

func (g *GroupContactDetailModel) Username() string {
	return g.UserName.Str
}

func (g *GroupContactDetailModel) Nickname() string {
	return g.NickName.Str
}

func (g *GroupContactDetailModel) AvatarUrl() string {
	return g.SmallHeadImgUrl
}

// more room specific fields
func (w *WechatClient) GetChatRoomInfo(rooms ...string) (*GetContactDetailsApiResult[GroupContactDetailModel], error) {
	res := &GetContactDetailsApiResult[GroupContactDetailModel]{}
	if _, err := w.doPostAPICall("/group/GetChatRoomInfo", &GetChatRoomInfoModel{
		ChatRoomWxIdList: rooms,
	}, res); err != nil {
		return nil, err
	}
	return res, nil
}
