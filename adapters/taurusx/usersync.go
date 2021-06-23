package taurusx

import (
	"text/template"

	"github.com/prebid/prebid-server/adapters"
	"github.com/prebid/prebid-server/usersync"
)

// NewTaurusXSyncer ...
func NewTaurusXSyncer(template *template.Template) usersync.Usersyncer {
	return adapters.NewSyncer("taurusx", template, adapters.SyncTypeRedirect)
}
