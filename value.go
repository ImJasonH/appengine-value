// +build appengine

// Package value provides a utility method to retrieve string values in an
// App Engine app, so they can be stored in the datastore instead of in source.
//
// Values can be added using the admin interface served at /_ah/value/admin
// (app.yaml must map this URL to script: _go_app to support this), or by using
// the App Engine Datastore Viewer UI.
//
// Once retrieved, values will be cached in memcache and local instance
// memory for quick lookup. Values cannot be updated, except via the Admin UI.
//
// Intended use cases are OAuth client secrets or API keys, for instance, which
// are used across many requests are should be quick to look up, but shouldn't
// be stored in source control as consts for security reasons.
//
// Values are not encrypted or obfuscated, and will be easily visible to any
// other app admin.
package value

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"appengine"
	"appengine/datastore"
	"appengine/memcache"
	"appengine/user"
)

// Kind is the name used to store values in the datastore.
var Kind = "Values"

// Prefix to use when storing values in memcache.
var MemcacheKeyPrefix = ""

var local = map[string]string{}

type e struct {
	Value string `datastore:",noindex"`
}

// Get returns the value associated with the key.
//
// If the value cannot be found in any of local instance memory, memcache, or
// the datastore, an empty string will be returned.
func Get(c appengine.Context, key string) string {
	if v, ok := local[key]; ok {
		// Found value in instance memory, return it.
		return v
	}

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

	// Store value in instance memory for next time.
	local[key] = e.Value

	// Store value in memcache for next time.
	if err := memcache.Set(c, &memcache.Item{
		Key:   MemcacheKeyPrefix + key,
		Value: []byte(e.Value),
	}); err != nil {
		c.Errorf("error setting %q in memcache: %v", key, err)
	}

	return e.Value
}

func init() {
	http.HandleFunc("/_ah/value/admin", adminHandler)
	http.HandleFunc("/_ah/value/update", updateHandler)
}

var adminTmpl = template.Must(template.New("admin").Parse(`<html><body>
<h1>Admin</h1>
<form action="/_ah/value/update" method="POST">
<table>
{{range $key, $val := .}}
  <tr><td>{{$key}}</td><td>{{$val}}</td><tr>
{{else}}
<b>no values currently configured</b><br />
{{end}}
<tr><td><input type="text" name="key"></input></td><td><input type="text" name="val"></input></td></tr>
</table>
<input type="submit" value="Save"></input>
</form>
</body></html>`))

func adminHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	if !user.IsAdmin(c) {
		if user.Current(c) == nil {
			loginURL, _ := user.LoginURL(c, "/_ah/value/admin")
			fmt.Fprintf(w, "<a href='%s'>Log in</a>", loginURL)
		} else {
			http.Error(w, "Forbidden", http.StatusForbidden)
		}
		return
	}
	if r.Method != "GET" {
		return
	}
	v := map[string]string{}
	q := datastore.NewQuery(Kind)
	for t := q.Run(c); ; {
		var e e
		k, err := t.Next(&e)
		if err == datastore.Done {
			break
		} else if err != nil {
			c.Errorf("%+v", err)
			return
		}
		v[k.StringID()] = e.Value
	}

	// TODO
	adminTmpl.Execute(w, v)
}

func updateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/_ah/value/admin", http.StatusSeeOther)
		return
	}
	c := appengine.NewContext(r)
	if !user.IsAdmin(c) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	key := r.FormValue("key")
	val := r.FormValue("val")
	if err := set(c, key, val); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// TODO: remove this hack.
	time.Sleep(time.Millisecond * 500)
	http.Redirect(w, r, "/_ah/value/admin", http.StatusSeeOther)
}

func set(c appengine.Context, key string, val string) error {
	if _, ok := local[key]; ok {
		return errors.New("key found in local instance")
	}
	if _, err := memcache.Get(c, key); err != memcache.ErrCacheMiss {
		return errors.New("key found in memcache")
	}
	return datastore.RunInTransaction(c, func(tc appengine.Context) error {
		// Fail if the value is already stored.
		k := datastore.NewKey(c, Kind, key, 0, nil)
		if err := datastore.Get(tc, k, nil); err == nil {
			return errors.New("key found in datastore")
		} else if err == datastore.ErrNoSuchEntity {
			// Okay, good. Keep going.
		} else if err != nil {
			return err
		}

		// Put the value in the datastore.
		_, err := datastore.Put(tc, k, &e{Value: val})
		return err
	}, nil)
}
