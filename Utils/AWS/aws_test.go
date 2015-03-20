package AWS

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAPNSToken(t *testing.T) {
	assert := assert.New(t)

	pushb64 := "pOsktxj+u6C/w7Tew4bIEiafcB4ZkBnDdbG4y/yLLoA="
	pushraw := "BEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEE"

	token, err := DecodeAPNSPushToken(pushb64)
	assert.NotEmpty(token)
	assert.NoError(err)
	assert.Equal(64, len(token))

	token, err = DecodeAPNSPushToken(pushraw)
	assert.NotEmpty(token)
	assert.Equal(token, pushraw)
	assert.NoError(err)

	token, err = DecodeAPNSPushToken("!@#$%")
	assert.Empty(token)
	assert.Error(err)

}
