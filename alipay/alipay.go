package alipay

import (
	"fmt"
	"net/url"
	"time"

	"github.com/Centny/gwf/util"

	"github.com/Centny/gwf/log"
	"github.com/Centny/gwf/routing"
)

type Evh interface {
	OnReturn(c *Client, hs *routing.HTTPSession) routing.HResult
	OnNotify(c *Client, hs *routing.HTTPSession) error
}
type Client struct {
	Gateway string
	Web     Conf
	Mobile  Conf
	H       Evh
}

func NewClient(gateway string, h Evh) *Client {
	return &Client{Gateway: gateway, H: h}
}

func (c *Client) CreateUrl(utype, notify_url, return_url, out_trade_no, subject, body string, total_fee float64) string {
	var vals = &url.Values{}
	switch utype {
	case "PRE":
		vals.Add("app_id", c.Web.Appid)
		vals.Add("method", "alipay.trade.precreate")
		vals.Add("format", "JSON")
		vals.Add("charset", "utf-8")
		vals.Add("timestamp", time.Now().Format("2006-01-02 15:04:05"))
		vals.Add("version", "1.0")
		vals.Add("notify_url", notify_url)
		vals.Add("biz_content", util.S2Json(util.Map{
			"out_trade_no": out_trade_no,
			"total_amount": fmt.Sprintf("%.02f", total_fee),
			"subject":      subject,
		}))
		//
		vals.Add("sign_type", "RSA")
		var data = vals.Encode()
		data, _ = url.QueryUnescape(data)
		var sign, _ = c.Web.ShaSign(data)
		vals.Add("sign", sign)
		return fmt.Sprintf("%v?%v", c.Gateway, vals.Encode())
		//
	case "APP":
		vals.Add("partner", fmt.Sprintf("\"%v\"", c.Web.Partner))
		vals.Add("seller_id", fmt.Sprintf("\"%v\"", c.Web.Seller))
		vals.Add("service", fmt.Sprintf("\"%v\"", "mobile.securitypay.pay"))
		vals.Add("out_trade_no", fmt.Sprintf("\"%v\"", out_trade_no))
		vals.Add("subject", fmt.Sprintf("\"%v\"", subject))
		vals.Add("body", fmt.Sprintf("\"%v\"", body))
		vals.Add("total_fee", fmt.Sprintf("\"%.02f\"", total_fee))
		vals.Add("notify_url", fmt.Sprintf("\"%v\"", notify_url))
		vals.Add("payment_type", "\"1\"")
		vals.Add("_input_charset", "\"utf-8\"")
		//
		var data = vals.Encode()
		data, _ = url.QueryUnescape(data)
		var sign, _ = c.Web.ShaSign(data)
		vals.Add("sign_type", "\"RSA\"")
		vals.Add("sign", fmt.Sprintf("\"%v\"", url.QueryEscape(sign)))
		data, _ = url.QueryUnescape(vals.Encode())
		return data
		//
	default:
		vals.Add("_input_charset", "utf-8")
		vals.Add("service", "create_direct_pay_by_user")
		vals.Add("partner", c.Web.Partner)
		vals.Add("notify_url", notify_url)
		if len(return_url) > 0 {
			vals.Add("return_url", return_url)
		}
		vals.Add("out_trade_no", out_trade_no)
		vals.Add("subject", subject)
		vals.Add("payment_type", "1")
		vals.Add("total_fee", fmt.Sprintf("%.02f", total_fee))
		vals.Add("seller_email", c.Web.Seller)
		vals.Add("body", body)
		//
		var data = vals.Encode()
		data, _ = url.QueryUnescape(data)
		var sign = c.Web.Md5Sign(data)
		vals.Add("sign_type", "MD5")
		vals.Add("sign", sign)
		return fmt.Sprintf("%v?%v", c.Gateway, vals.Encode())
		//
	}
}

func (c *Client) Return(hs *routing.HTTPSession) routing.HResult {
	var addr = hs.R.Header.Get("X-Real-IP")
	if len(addr) < 1 {
		addr = hs.R.RemoteAddr
	}
	hs.R.ParseForm()
	var vals = hs.R.Form
	var sign, sign_type = vals.Get("sign"), vals.Get("sign_type")
	vals.Del("sign")
	vals.Del("sign_type")
	var data = vals.Encode()
	data, _ = url.QueryUnescape(data)
	var err = c.Web.Verify(data, sign, sign_type)
	if err == nil {
		slog("Client.Return(Alipay) receive verify request from(%v) and call on return by args:\nsign_type=%v&sign=%v%v\n<-", addr, sign_type, sign, data)
		return c.H.OnReturn(c, hs)
	} else {
		log.W("Client.Notify(Alipay) recieve bad request from address(%v),err:%v->\nsign_type=%v&sign=%v&%v", addr, err, sign_type, sign, data)
		hs.W.WriteHeader(400)
		hs.W.Write([]byte(err.Error()))
		return routing.HRES_RETURN
	}
}

func (c *Client) Notify(hs *routing.HTTPSession) routing.HResult {
	var addr = hs.R.Header.Get("X-Real-IP")
	if len(addr) < 1 {
		addr = hs.R.RemoteAddr
	}
	hs.R.ParseForm()
	var vals = hs.R.Form
	var sign, sign_type = vals.Get("sign"), vals.Get("sign_type")
	vals.Del("sign")
	vals.Del("sign_type")
	var data = vals.Encode()
	data, _ = url.QueryUnescape(data)
	var err = c.Web.Verify(data, sign, sign_type)
	if err != nil {
		log.W("Client.Notify(Alipay) recieve bad request from address(%v),err:%v->\nsign_type=%v&sign=%v&%v", addr, err, sign_type, sign, data)
		hs.W.WriteHeader(400)
		hs.W.Write([]byte(err.Error()))
		return routing.HRES_RETURN
	}
	slog("Client.Notify(Alipay) receive verify request from address(%v) by args:\nsign_type=%v&sign=%v&%v\n<-", addr, sign_type, sign, data)
	err = c.H.OnNotify(c, hs)
	if err == nil {
		hs.W.Write([]byte("success"))
	} else {
		log.W("Client.Notify(Alipay) call on notify fail with error(%v)", err)
		hs.W.WriteHeader(400)
		hs.W.Write([]byte(err.Error()))
	}
	return routing.HRES_RETURN
}
