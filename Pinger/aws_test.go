package Pinger

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAPNSToken(t *testing.T) {
	assert := assert.New(t)

	pushb64 := "pOsktxj+u6C/w7Tew4bIEiafcB4ZkBnDdbG4y/yLLoA="
	pushraw := "BEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEE"
	
	token, err := decodeAPNSPushToken(pushb64)
	assert.NotEmpty(token)
	assert.NoError(err)

	token, err = decodeAPNSPushToken(pushraw)
	assert.NotEmpty(token)
	assert.Equal(token, pushraw)
	assert.NoError(err)

	token, err = decodeAPNSPushToken("!@#$%")
	assert.Empty(token)
	assert.Error(err)

}
