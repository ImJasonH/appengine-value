// +build appengine

// Package value provides a utility methods to retrieve string values in an
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
	return GetMulti(c, key)[key]
}

// GetMulti is a batch version of Get. It returns a map keyed on the provided keys.
//
// If a key is not found in memcache or datastore, it will map to an empty string.
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
	fromDS := make([]e, len(keys))
	if err := datastore.GetMulti(c, keys, fromDS); err != nil {
		// TODO: appengine.MultiError may contain only ErrNoSuchEntity errs,
		// in which case we should populate as many results as exist. If any
		// are not ErrNoSuchEntity then something actually went wrong.
		c.Errorf("error getting multi from datastore: %v", err)
		return m
	}
	items := []*memcache.Item{}
	for i, de := range fromDS {
		m[keys[i].StringID()] = de.Value
		items = append(items, &memcache.Item{
			Key:   MemcacheKeyPrefix + keys[i].StringID(),
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

// String defines a string value with specified name.
// The return value is the address of a string variable that stores the value when Init is called.
func String(key string) *string {
	p := new(string)
	StringVar(p, key)
	return p
}

// StringVar defines a string value with specified name.
// The argument p points to a string variable in which to store the value when Init is called.
func StringVar(p *string, key string) {
	vals[key] = p
}

// Init populates values defined using String or StringVar.
// Must be called after all values are defined and before values are accessed by the program.
func Init(c appengine.Context) {
	keys := make([]string, len(vals))
	i := 0
	for k, _ := range vals {
		keys[i] = k
		i++
	}
	m := GetMulti(c, keys...)
	for k, v := range m {
		vals[k] = &v
	}
}
