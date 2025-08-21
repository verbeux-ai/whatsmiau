package controllers

import (
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow/types"
)

func numberToJid(number string) (*types.JID, error) {
	splitNumber := strings.Split(number, "@")
	if len(splitNumber) != 2 {
		number += "@s.whatsapp.net"
	}

	if len(splitNumber[0]) < 12 {
		return nil, fmt.Errorf("invalid jid, put country prefix")
	}

	jid, err := types.ParseJID(number)
	if err != nil {
		return nil, fmt.Errorf("invalid jid (number)")
	}

	return &jid, nil
}
