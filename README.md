`appengine-value` is a simple utility library to handle simple secret configuration values in an App Engine app.

Problem
-----

Your app requires secret values (OAuth secrets, API keys, passwords) and you rightly don't want to specify these values as consts in your source code.

**Don't do this!**

```
const (
	clientID     = "123456.clientaccount.foo"
	clientSecret = "s8p3rs3cr1t"
)
```

Solution
-----

**Do this instead**
```
import value "github.com/ImJasonH/appengine-value"

func doOAuth(c appengine.Context) {
	clientID := value.Get(c, "client_id")
	clientSecret := value.Get(c, "client_secret")
	// use clientID and clientSecret
}
```

If you have multiple values, you can **batch lookups**:
```
func doOAuth(c appengine.Context) {
	vals := value.GetMulti(c, "client_id", "client_secret")
	// use vals["clientID"] and vals["clientSecret"]
}
```

**Or define values like flags**
```
var (
	clientID     = value.String("client_id")
	clientSecret = value.String("client_secret")
)

func doOAuth(c appengine.Context) {
	value.Init(c)
	// use *clientID and *clientSecret
}
```

Configuration
-----

To set or unset values, use the admin UI which is served at `/_ah/value/admin` -- be sure to map this in your `app.yaml`:

```
handlers:
- url: /_ah/value/.*
  script: _go_app
  login: admin
```

(You can always remove the URL mapping if you're done setting values)
