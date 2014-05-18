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

func GetMulti(c appengine.Context, key ...string) map[string]string {
	m := map[string]string{}

	// Get whatever values we can from memcache
	mi, err := memcache.GetMulti(c, key)
	if err != nil {
		c.Errorf("error getting multi from memcache: %v", err)
	}
	for k, i := range mi {
		m[k[len(MemcacheKeyPrefix):]] = string(i.Value)
	}
	if len(mi) == len(key) {
		// All values found in memcahe!
		return m
	}

	// Get values not found in memcache from datastore.
	keys := []*datastore.Key{}
	for _, k := range key {
		if _, ok := mi[k]; !ok {
			keys = append(keys, datastore.NewKey(c, Kind, k, 0, nil))
		}
	}
	fromDS := []e{}
	if err := datastore.GetMulti(c, keys, fromDS); err != nil {
		c.Errorf("error getting multi from datastore: %v", err)
		return map[string]string{}
	}
	items := []*memcache.Item{}
	for i, de := range fromDS {
		m[keys[i].StringID()] = de.Value
		items = append(items, &memcache.Item{
			Key:   MemcacheKeyPrefix+keys[i].StringID(),
			Value: []byte(de.Value),
		})
	}

	// Store values in memcache for next time.
	if err := memcache.SetMulti(c, items); err != nil {
		c.Errorf("error setting multi in memcache: %v", err)
	}
	return m
}

var vals = map[string]*string{}

func String(key string) *string {
	p := new(string)
	StringVar(p, key)
	return p
}

func StringVar(p *string, key string) {
	vals[key] = p
}

func Init(c appengine.Context) {
	// TODO: GetMulti
	for k, _ := range vals {
		v := Get(c, k)
		vals[k] = &v
	}
}
