package domain

import "fmt"

type Device struct {
	N         int    `csv:"n"          db:"n"          json:"n"`
	MQTT      string `csv:"mqtt"       db:"mqtt"       json:"mqtt"`
	InvID     string `csv:"invid"      db:"inv_id"     json:"inv_id"`
	UnitGUID  string `csv:"unit_guid"  db:"unit_guid"  json:"unit_guid"`
	MsgID     string `csv:"msg_id"     db:"msg_id"     json:"msg_id"`
	Text      string `csv:"text"       db:"text"       json:"text"`
	Context   string `csv:"context"    db:"context"    json:"context"`
	Class     string `csv:"class"      db:"class"      json:"class"`
	Level     int    `csv:"level"      db:"level"      json:"level"`
	Area      string `csv:"area"       db:"area"       json:"area"`
	Addr      string `csv:"addr"       db:"addr"       json:"addr"`
	Block     string `csv:"block"      db:"block"      json:"block"`
	Type      string `csv:"type"       db:"type"       json:"type"`
	Bit       string `csv:"bit"        db:"bit"        json:"bit"`
	InvertBit string `csv:"invert_bit" db:"invert_bit" json:"invert_bit"`
}

func (d *Device) Validate() error {
	if d.UnitGUID == "" {
		return fmt.Errorf("unit_guid is required")
	}
	
	if d.N == 0 {
		return fmt.Errorf("n is required")
	}

	if d.Class == "" {
		return fmt.Errorf("class is required")
	}

	return nil
}
