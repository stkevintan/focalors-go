package wechat

type UserInfo struct {
	UserName   StrWrapper `json:"userName"`
	NickName   StrWrapper `json:"nickName"`
	BindMobile StrWrapper `json:"bindMobile"`
}
type UserInfoExt struct {
	// SnsUserInfo struct {
	// 	SnsFlag          int    `json:"sns_flag"`
	// 	SnsBgimgId       string `json:"sns_bgimg_id"`
	// 	SnsBgobjectId    int    `json:"sns_bgobject_id"`
	// 	SnsFlagex        int    `json:"sns_flagex"`
	// 	SnsPrivacyRecent int    `json:"sns_privacy_recent"`
	// } `json:"snsUserInfo"`
	BigHeadImgUrl   string `json:"bigHeadImgUrl"`
	SmallHeadImgUrl string `json:"smallHeadImgUrl"`
	// MainAcctType    int    `json:"mainAcctType"`
	// SafeDeviceList  struct {
	// 	Count int `json:"count"`
	// } `json:"safeDeviceList"`
	// SafeDevice      int    `json:"safeDevice"`
	// GrayscaleFlag   int    `json:"grayscaleFlag"`
	// RegCountry      string `json:"regCountry"`
	// PatternLockInfo struct {
	// 	PatternVersion int `json:"patternVersion"`
	// 	Sign           struct {
	// 		Len    int    `json:"len"`
	// 		Buffer string `json:"buffer"`
	// 	} `json:"sign"`
	// 	LockStatus int `json:"lockStatus"`
	// } `json:"patternLockInfo"`
	// PayWalletType int `json:"payWalletType"`
	// WalletRegion  int `json:"walletRegion"`
	// ExtStatus     int `json:"extStatus"`
	// UserStatus    int `json:"userStatus"`
}

type UserProfile struct {
	UserInfo    UserInfo    `json:"userInfo"`
	UserInfoExt UserInfoExt `json:"userInfoExt"`
}

func (w *WechatClient) GetProfile() (profile *UserProfile, err error) {
	res := &UserProfile{}
	if _, err := w.doGetAPICall("/user/GetProfile", res); err != nil {
		return nil, err
	}
	return res, nil
}
