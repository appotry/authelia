package handlers

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/authelia/authelia/v4/internal/duo"
	"github.com/authelia/authelia/v4/internal/mocks"
	"github.com/authelia/authelia/v4/internal/model"
)

type RegisterDuoDeviceSuite struct {
	suite.Suite
	mock *mocks.MockAutheliaCtx
}

func (s *RegisterDuoDeviceSuite) SetupTest() {
	s.mock = mocks.NewMockAutheliaCtx(s.T())
	userSession := s.mock.Ctx.GetSession()
	userSession.Username = testUsername
	err := s.mock.Ctx.SaveSession(userSession)
	s.Assert().NoError(err)
}

func (s *RegisterDuoDeviceSuite) TearDownTest() {
	s.mock.Close()
}

func (s *RegisterDuoDeviceSuite) TestShouldCallDuoAPIAndFail() {
	duoMock := mocks.NewMockAPI(s.mock.Ctrl)

	values := url.Values{}
	values.Set("username", "john")

	duoMock.EXPECT().PreAuthCall(s.mock.Ctx, gomock.Eq(values)).Return(nil, fmt.Errorf("Connnection error"))

	SecondFactorDuoDevicesGet(duoMock)(s.mock.Ctx)

	s.mock.Assert200KO(s.T(), "Authentication failed, please retry later.")
	assert.Equal(s.T(), "duo PreAuth API errored: Connnection error", s.mock.Hook.LastEntry().Message)
	assert.Equal(s.T(), logrus.ErrorLevel, s.mock.Hook.LastEntry().Level)
}

func (s *RegisterDuoDeviceSuite) TestShouldRespondWithSelection() {
	duoMock := mocks.NewMockAPI(s.mock.Ctrl)

	var duoDevices = []duo.Device{
		{Capabilities: []string{"auto", "push", "sms", "mobile_otp"}, Number: " ", Device: "12345ABCDEFGHIJ67890", DisplayName: "Test Device 1"},
		{Capabilities: []string{"auto", "push", "sms", "mobile_otp"}, Number: "+123456789****", Device: "1234567890ABCDEFGHIJ", DisplayName: "Test Device 2"},
		{Capabilities: []string{"auto", "sms", "mobile_otp"}, Number: "+123456789****", Device: "1234567890ABCDEFGHIJ", DisplayName: "Test Device 3"},
	}

	var apiDevices = []DuoDevice{
		{Capabilities: []string{"push"}, Device: "12345ABCDEFGHIJ67890", DisplayName: "Test Device 1"},
		{Capabilities: []string{"push"}, Device: "1234567890ABCDEFGHIJ", DisplayName: "Test Device 2"},
	}

	values := url.Values{}
	values.Set("username", "john")

	response := duo.PreAuthResponse{}
	response.Result = auth
	response.Devices = duoDevices

	duoMock.EXPECT().PreAuthCall(s.mock.Ctx, gomock.Eq(values)).Return(&response, nil)

	SecondFactorDuoDevicesGet(duoMock)(s.mock.Ctx)

	s.mock.Assert200OK(s.T(), DuoDevicesResponse{Result: auth, Devices: apiDevices})
}

func (s *RegisterDuoDeviceSuite) TestShouldRespondWithAllowOnBypass() {
	duoMock := mocks.NewMockAPI(s.mock.Ctrl)

	values := url.Values{}
	values.Set("username", "john")

	response := duo.PreAuthResponse{}
	response.Result = allow

	duoMock.EXPECT().PreAuthCall(s.mock.Ctx, gomock.Eq(values)).Return(&response, nil)

	SecondFactorDuoDevicesGet(duoMock)(s.mock.Ctx)

	s.mock.Assert200OK(s.T(), DuoDevicesResponse{Result: allow})
}

func (s *RegisterDuoDeviceSuite) TestShouldRespondWithEnroll() {
	duoMock := mocks.NewMockAPI(s.mock.Ctrl)

	var enrollURL = "https://api-example.duosecurity.com/portal?code=1234567890ABCDEF&akey=12345ABCDEFGHIJ67890"

	values := url.Values{}
	values.Set("username", "john")

	response := duo.PreAuthResponse{}
	response.Result = enroll
	response.EnrollPortalURL = enrollURL

	duoMock.EXPECT().PreAuthCall(s.mock.Ctx, gomock.Eq(values)).Return(&response, nil)

	SecondFactorDuoDevicesGet(duoMock)(s.mock.Ctx)

	s.mock.Assert200OK(s.T(), DuoDevicesResponse{Result: enroll, EnrollURL: enrollURL})
}

func (s *RegisterDuoDeviceSuite) TestShouldRespondWithDeny() {
	duoMock := mocks.NewMockAPI(s.mock.Ctrl)

	values := url.Values{}
	values.Set("username", "john")

	response := duo.PreAuthResponse{}
	response.Result = deny

	duoMock.EXPECT().PreAuthCall(s.mock.Ctx, gomock.Eq(values)).Return(&response, nil)

	SecondFactorDuoDevicesGet(duoMock)(s.mock.Ctx)

	s.mock.Assert200OK(s.T(), DuoDevicesResponse{Result: deny})
}

func (s *RegisterDuoDeviceSuite) TestShouldRespondOK() {
	s.mock.Ctx.Request.SetBodyString("{\"device\":\"1234567890123456\", \"method\":\"push\"}")
	s.mock.StorageMock.EXPECT().
		SavePreferredDuoDevice(gomock.Eq(s.mock.Ctx), gomock.Eq(model.DuoDevice{Username: "john", Device: "1234567890123456", Method: "push"})).
		Return(nil)

	SecondFactorDuoDevicePost(s.mock.Ctx)

	assert.Equal(s.T(), 200, s.mock.Ctx.Response.StatusCode())
}

func (s *RegisterDuoDeviceSuite) TestShouldRespondKOOnInvalidMethod() {
	s.mock.Ctx.Request.SetBodyString("{\"device\":\"1234567890123456\", \"method\":\"testfailure\"}")

	SecondFactorDuoDevicePost(s.mock.Ctx)

	s.mock.Assert200KO(s.T(), "Authentication failed, please retry later.")
	assert.Equal(s.T(), logrus.ErrorLevel, s.mock.Hook.LastEntry().Level)
}

func (s *RegisterDuoDeviceSuite) TestShouldRespondKOOnEmptyMethod() {
	s.mock.Ctx.Request.SetBodyString("{\"device\":\"1234567890123456\", \"method\":\"\"}")

	SecondFactorDuoDevicePost(s.mock.Ctx)

	s.mock.Assert200KO(s.T(), "Authentication failed, please retry later.")
	assert.Equal(s.T(), "unable to validate body: method: non zero value required", s.mock.Hook.LastEntry().Message)
	assert.Equal(s.T(), logrus.ErrorLevel, s.mock.Hook.LastEntry().Level)
}

func (s *RegisterDuoDeviceSuite) TestShouldRespondKOOnEmptyDevice() {
	s.mock.Ctx.Request.SetBodyString("{\"device\":\"\", \"method\":\"push\"}")

	SecondFactorDuoDevicePost(s.mock.Ctx)

	s.mock.Assert200KO(s.T(), "Authentication failed, please retry later.")
	assert.Equal(s.T(), "unable to validate body: device: non zero value required", s.mock.Hook.LastEntry().Message)
	assert.Equal(s.T(), logrus.ErrorLevel, s.mock.Hook.LastEntry().Level)
}

func TestRunRegisterDuoDeviceSuite(t *testing.T) {
	s := new(RegisterDuoDeviceSuite)
	suite.Run(t, s)
}
