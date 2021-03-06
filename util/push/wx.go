package push

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/prometheus/common/log"
	"golang.org/x/xerrors"
)

const (
	POST = iota
	GET
)

type PostMessage struct {
	AppToken    string   `json:"appToken"`
	Content     string   `json:"content"`
	Summary     string   `json:"summary"`
	ContentType int      `json:"contentType"`
	Uids        []string `json:"uids"`
	Url         string   `json:"url"`
}

func WxPush(msg PostMessage) error {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("WxPush panic error %v", err)
		}
	}()
	msg.AppToken = PUSHTOKEN
	msg.Uids = UIDS
	msg.ContentType = 1
	msgbuf, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	resp, err := http.Post(PUSHURL, "application/json", bytes.NewBuffer(msgbuf))
	if err != nil {
		return err
	}
	defer func() {
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var ret map[string]interface{}
	err = json.Unmarshal(buf, &ret)
	if err != nil {
		return err
	}
	if ret["code"].(float64) != 1000 {
		log.Error(ret)
		return xerrors.Errorf("push message failed")
	}
	log.Infof("wx push message %v", msg)
	return nil
}
