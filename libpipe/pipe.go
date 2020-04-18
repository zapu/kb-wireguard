package libpipe

import "encoding/json"

type PipeMsg struct {
	ID      string `json:"i"`
	Payload string `json:"p"`
}

func SerializeMsgInterface(ID string, obj interface{}) (ret string, err error) {
	objBytes, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	return SerializeMsgString(ID, string(objBytes))
}

func SerializeMsgString(ID string, val string) (ret string, err error) {
	pMsg := PipeMsg{
		ID:      ID,
		Payload: val,
	}
	b, err := json.Marshal(pMsg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (pmsg *PipeMsg) DeserializePayload(out interface{}) error {
	return json.Unmarshal([]byte(pmsg.Payload), out)
}
