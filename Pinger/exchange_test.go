package Pinger

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewExchangeClient(t *testing.T) {
	assert := assert.New(t)

	parent := &MailClientContext{}
	debug := true

	ex, err := NewExchangeClient(parent, debug, logger)
	assert.NoError(err)
	assert.NotNil(ex)

	assert.NotNil(ex.parent)
	assert.Equal(parent, ex.parent)
	assert.NotNil(ex.incoming)
	assert.Equal(debug, ex.debug)
	assert.Equal(logger, ex.logger)
}
