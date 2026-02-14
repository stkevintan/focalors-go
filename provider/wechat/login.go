package wechat

type LoginStatusApiResult struct {
	ApiResult
	Data struct {
		ExpiryTime   string `json:"expiryTime"`
		LoginErrMsg  string `json:"loginErrMsg"`
		LoginState   int    `json:"loginState"`
		OnlineDays   int    `json:"onlineDays"`
		TotalOneline string `json:"totalOneline"`
		ProxyUrl     string `json:"proxyUrl"`
		TargetIp     string `json:"targetIp"`
	} `json:"Data"`
}

/*
	{
	  "Code": 300,
	  "Data": null,
	  "Text": "该账号需要重新登录！loginState == MMLoginStateNoLogin "
	}

	{
	  "Code": 200,
	  "Data": {
	    "expiryTime": "2025-07-03",
	    "loginErrMsg": "账号在线状态良好！",
	    "loginJournal": {
	      "count": 0,
	      "logs": []
	    },
	    "loginState": 1,
	    "loginTime": "2025-06-03 06:37:08",
	    "onlineDays": 3,
	    "onlineTime": "本次在线: 3天6时51分",
	    "proxyUrl": "",
	    "targetIp": "153.35.129.254:1239",
	    "totalOnline": "总计在线: 3天6时51分"
	  },
	  "Text": ""
	}
*/
func (w *WechatClient) GetLoginStatus() (*LoginStatusApiResult, error) {
	res := &LoginStatusApiResult{}
	if _, err := w.doGetAPICall("/login/GetLoginStatus", res); err != nil {
		return nil, err
	}
	return res, nil
}

type QRCodeApiResult struct {
	ApiResult
	Data struct {
		QrCodeUrl string `json:"QrCodeUrl"`
		Key       string `json:"Key"`
		Txt       string `json:"Txt"`
	} `json:"Data"`
}

type GetLoginQrCodeModel struct {
	Proxy string // socks代理，例如：socks5://username:password@ipv4:port
	Check bool   `example:"false"` // 修改代理时(SetProxy接口) 是否发送检测代理请求(可能导致请求超时)
}

func (w *WechatClient) GetLoginQRCode() (*QRCodeApiResult, error) {
	res := &QRCodeApiResult{}
	if _, err := w.doPostAPICall("/login/GetLoginQrCodeNew", GetLoginQrCodeModel{}, res); err != nil {
		return nil, err
	}

	return res, nil
}

func (w *WechatClient) WakeUpLogin() (*ApiResult, error) {
	res := &ApiResult{}
	if _, err := w.doPostAPICall("/login/WakeUpLogin", GetLoginQrCodeModel{}, res); err != nil {
		return nil, err
	}

	return res, nil
}
