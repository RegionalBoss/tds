package tds

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net/url"
	"strconv"
)

const defaultCharset = "utf8"
const defaultTextSize = 32768

// connection Timeout in seconds
const defaultLoginTimeout = 20

type connParams struct {
	host         string
	user         string
	password     string
	clientHost   string // client host name
	app          string // client application name
	charset      string
	packetSize   int
	loginTimeout int    // login Timeout
	readTimeout  int    // read Timeout
	writeTimeout int    // write Timeout
	database     string // if requested at connection time
	pid          string
	textSize     int
	ssl          string
	// yes: mandatory password encryption.
	// no: never encrypt password.
	// try: try encryption, fallback to non encrypted password.
	encryptPassword string
}

// Conn encapsulates a tds session and satisties driver.Connc
type Conn struct {
	*session
}

// parse the DSN given by the user
func parseDSN(dsn string) (prm connParams, err error) {
	url, err := url.Parse(dsn)
	if err != nil {
		return prm, err
	}

	// get server / database
	prm.host = url.Host
	if len(url.Path) > 1 {
		prm.database = url.Path[1:len(url.Path)]
	}

	// user/pass
	if url.User != nil {
		prm.user = url.User.Username()
		prm.password, _ = url.User.Password()
	}

	// additionnal parameters
	values := url.Query()
	prm.packetSize, _ = strconv.Atoi(values.Get("packetSize"))
	if prm.packetSize == 0 {
		prm.packetSize = 512
	}

	// get login, read and write Timeouts
	prm.loginTimeout, err = strconv.Atoi(values.Get("loginTimeout"))
	if err != nil || prm.loginTimeout <= 0 {
		prm.loginTimeout = defaultLoginTimeout
	}

	prm.readTimeout, err = strconv.Atoi(values.Get("readTimeout"))
	prm.writeTimeout, err = strconv.Atoi(values.Get("writeTimeout"))

	// get password encryption method
	if values.Get("encryptPassword") == "" {
		prm.encryptPassword = "try"
	}

	if prm.encryptPassword != "yes" &&
		prm.encryptPassword != "no" &&
		prm.encryptPassword != "try" {
		return prm, fmt.Errorf("tds: encryptPassword must be 'yes', 'no' or 'try'")
	}

	// ssl ??
	if values.Get("ssl") == "on" {
		prm.ssl = "on"
	}

	switch values.Get("charset") {
	case "none":
		prm.charset = ""
	case "utf8", "utf-8", "UTF8", "UTF-8", "":
		prm.charset = "utf8"
	default:
		prm.charset = values.Get("charset")
	}

	prm.app = values.Get("applicationName")
	prm.clientHost = values.Get("hostName")
	prm.pid = values.Get("pid")
	prm.textSize, _ = strconv.Atoi(values.Get("textSize"))
	if err != nil {
		prm.textSize = defaultTextSize
	}

	// mandatory parameters
	if prm.host == "" {
		return prm, errors.New("tds: connect failed. Please specify hostname")
	}
	if prm.user == "" {
		return prm, errors.New("tds: connect failed. Please specify user")
	}
	if validHost.FindString(prm.host) == "" {
		return prm, errors.New("tds: connect failed. Please specify host name in the form host:port")
	}

	if prm.packetSize != 512 && prm.packetSize != 1024 &&
		prm.packetSize != 2048 && prm.packetSize != 4096 {
		return prm, errors.New("tds: invalid packet size. must be 512, 1024, 2048 or 4096")
	}

	return prm, nil
}

// NewConn returns a TDS session
func NewConn(dsn string) (*Conn, error) {
	prm, err := parseDSN(dsn)
	if err != nil {
		return nil, err
	}
	s, err := newSession(prm)
	if err != nil {
		return nil, err
	}
	if s.isBad {
		return nil, driver.ErrBadConn
	}

	c := &Conn{session: s}
	return c, nil
}

// SetErrorhandler allows setting a custom error handler.
// The function shall accept an SQL Message and return a boolean
// indicating if this message is indeed a critical error.
func (c *Conn) SetErrorhandler(fn func(s SybError) bool) {
	c.session.IsError = fn
}

// GetEnv return a map of environments variables.
// The following keys are garanteed to be present:
//  - server
//  - database
//  - charset
func (c Conn) GetEnv() map[string]string {
	return map[string]string{
		"server":     c.session.serverType,
		"serverType": c.session.serverType,
		"database":   c.session.database,
		"charset":    c.session.charset,
	}
}

// register the driver
type sybDriver struct{}

var sybDriverInstance = &sybDriver{}

func (d *sybDriver) Open(dsn string) (driver.Conn, error) {
	return NewConn(dsn)
}

func init() {
	sql.Register("syb", sybDriverInstance)
	sql.Register("tds", sybDriverInstance)
}

var _ driver.Driver = (*sybDriver)(nil)
