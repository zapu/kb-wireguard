package libpipe

import "encoding/json"

type PipeMsg struct {
	ID      string `json:"i"`
	Payload []byte `json:"p"`
}

func SerializeMsgInterface(ID string, obj interface{}) (ret string, err error) {
	objBytes, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	return SerializeMsgBytes(ID, objBytes)
}

func SerializeMsgString(ID string, val string) (ret string, err error) {
	return SerializeMsgBytes(ID, []byte(val))
}

func SerializeMsgBytes(ID string, val []byte) (ret string, err error) {
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
