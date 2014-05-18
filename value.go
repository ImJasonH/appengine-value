// +build appengine

// Package value provides a utility method to retrieve string values in an
// App Engine app, so they can be stored in the datastore instead of in source.
//
// Values can be added and removed using the admin interface served at
// /_ah/value/admin (app.yaml must map this URL to script: _go_app to support
// this), or by using the App Engine Datastore Viewer UI.
//
// Once retrieved, values will be cached in memcache for quick lookup.
//
// Intended use cases are OAuth client secrets or API keys, for instance, which
// are used across many requests are should be quick to look up, but shouldn't
// be stored in source control as consts for security reasons.
//
// Values are not encrypted or obfuscated, and will be easily visible to any
// other app admin.
package value

import (
	"appengine"
	"appengine/datastore"
	"appengine/memcache"
)

// Kind is the name used to store values in the datastore.
var Kind = "Values"

// Prefix to use when storing values in memcache.
var MemcacheKeyPrefix = ""

type e struct {
	Value string `datastore:",noindex"`
}

// Get returns the value associated with the key.
//
// If the value cannot be found in memcache or datastore, an empty string will
// be returned.
func Get(c appengine.Context, key string) string {
	i, err := memcache.Get(c, key)
	if err == nil {
		// Found value in memcache, return it.
		return string(i.Value)
	}

	if err != nil && err != memcache.ErrCacheMiss {
		c.Errorf("error getting %q from memcache: %v", key, err)
	}

	// Get value from datastore if missing from memcache.
	k := datastore.NewKey(c, Kind, key, 0, nil)
	var e e
	if err := datastore.Get(c, k, &e); err != nil {
		c.Errorf("error getting %q from datastore: %v", key, err)
		return ""
	}

	// Store value in memcache for next time.
	if err := memcache.Set(c, &memcache.Item{
		Key:   MemcacheKeyPrefix + key,
		Value: []byte(e.Value),
	}); err != nil {
		c.Errorf("error setting %q in memcache: %v", key, err)
	}

	return e.Value
}

var vals = map[string]*string{}

func String(key string) *string {
	p := new(string)
	vals[key] = p
	return p
}

func Init(c appengine.Context) {
	// TODO: GetMulti
	for k, _ := range vals {
		v := Get(c, k)
		vals[k] = &v
	}
}
