// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package api_test

import (
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/api"
	apitesting "github.com/juju/juju/api/testing"
)

var _ = gc.Suite(&macaroonLoginSuite{})

type macaroonLoginSuite struct {
	apitesting.MacaroonSuite
	client api.Connection
}

func (s *macaroonLoginSuite) SetUpTest(c *gc.C) {
	s.MacaroonSuite.SetUpTest(c)
	s.AddUser(c, "testuser")
	info := s.APIInfo(c)
	// Don't log in.
	info.UseMacaroons = false
	s.client = s.OpenAPI(c, info, nil)
}

func (s *macaroonLoginSuite) TearDownTest(c *gc.C) {
	s.client.Close()
	s.MacaroonSuite.TearDownTest(c)
}

func (s *macaroonLoginSuite) TestSuccessfulLogin(c *gc.C) {
	s.DischargerLogin = func() string { return "testuser" }
	err := s.client.Login(nil, "", "")
	c.Assert(err, jc.ErrorIsNil)
}

func (s *macaroonLoginSuite) TestFailedToObtainDischargeLogin(c *gc.C) {
	err := s.client.Login(nil, "", "")
	c.Assert(err, gc.ErrorMatches, `cannot get discharge from "https://.*": third party refused discharge: cannot discharge: login denied by discharger`)
}

func (s *macaroonLoginSuite) TestUnknownUserLogin(c *gc.C) {
	s.DischargerLogin = func() string {
		return "testUnknown"
	}
	err := s.client.Login(nil, "", "")
	c.Assert(err, gc.ErrorMatches, "invalid entity name or password")
}

func (s *macaroonLoginSuite) TestConnectStream(c *gc.C) {
	s.PatchValue(api.WebsocketDialConfig, echoURL(c))

	dischargeCount := 0
	s.DischargerLogin = func() string {
		dischargeCount++
		return "testuser"
	}
	// First log into the regular API.
	err := s.client.Login(nil, "", "")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(dischargeCount, gc.Equals, 1)

	// Then check that ConnectStream works OK and that it doesn't need
	// to discharge again.
	conn, err := s.client.ConnectStream("/path", nil)
	c.Assert(err, gc.IsNil)
	defer conn.Close()
	connectURL := connectURLFromReader(c, conn)
	c.Assert(connectURL.Path, gc.Equals, "/environment/"+s.State.EnvironTag().Id()+"/path")
	c.Assert(dischargeCount, gc.Equals, 1)
}

func (s *macaroonLoginSuite) TestConnectStreamWithoutLogin(c *gc.C) {
	s.PatchValue(api.WebsocketDialConfig, echoURL(c))

	conn, err := s.client.ConnectStream("/path", nil)
	c.Assert(err, gc.ErrorMatches, `cannot use ConnectStream without logging in`)
	c.Assert(conn, gc.Equals, nil)
}

func (s *macaroonLoginSuite) TestConnectStreamFailedDischarge(c *gc.C) {
	// This is really a test for ConnectStream, but to test ConnectStream's
	// discharge failing logic, we need an actual endpoint to test against,
	// and the debug-log endpoint makes a convenient example.

	var dischargeError bool
	s.DischargerLogin = func() string {
		if dischargeError {
			return ""
		}
		return "testuser"
	}

	// Make an API connection that uses a cookie jar
	// that allows us to remove all cookies.
	jar := apitesting.NewClearableCookieJar()
	client := s.OpenAPI(c, nil, jar)

	// Ensure that the discharger won't discharge and try
	// logging in again. We should succeed in getting past
	// authorization because we have the cookies (but
	// the actual debug-log endpoint will return an error
	// because there's no all-machines.log file).
	dischargeError = true
	conn, err := client.ConnectStream("/log", nil)
	c.Assert(err, gc.ErrorMatches, "cannot open log file: .*")
	c.Assert(conn, gc.Equals, nil)

	// Then delete all the cookies by deleting the cookie jar
	// and try again. The login should fail.
	jar.Clear()

	conn, err = client.ConnectStream("/log", nil)
	c.Assert(err, gc.ErrorMatches, `cannot get discharge from "https://.*": third party refused discharge: cannot discharge: login denied by discharger`)
	c.Assert(conn, gc.Equals, nil)
}