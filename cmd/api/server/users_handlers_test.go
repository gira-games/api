package server

import (
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/asankov/gira/internal/fixtures"
	"github.com/asankov/gira/pkg/models"
	"github.com/asankov/gira/pkg/models/postgres"
	"github.com/golang/mock/gomock"
)

var (
	expectedUser = models.User{
		Username: "test",
		Email:    "test@test.com",
		Password: "t3$T123",
	}
)

func setupUsersServer(u UserModel, a *fixtures.AuthenticatorMock) *Server {
	return &Server{
		Log:           log.New(os.Stdout, "", 0),
		UserModel:     u,
		Authenticator: a,
	}
}

func TestUserCreate(t *testing.T) {
	ctrl := gomock.NewController(t)

	userModel := fixtures.NewUserModelMock(ctrl)
	authenticator := fixtures.NewAuthenticatorMock(ctrl)
	srv := setupUsersServer(userModel, authenticator)

	userModel.EXPECT().
		Insert(&expectedUser).
		Return(&expectedUser, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/users", fixtures.Marshall(t, expectedUser))
	srv.ServeHTTP(w, r)

	got, expected := w.Code, http.StatusOK
	if got != expected {
		t.Fatalf("Got (%d) for status code, expected (%d)", got, expected)
	}

	var user models.User
	fixtures.Decode(t, w.Body, &user)
	if user.Username != expectedUser.Username {
		t.Errorf("Got (%s) for username, expected (%s)", user.Username, expectedUser.Username)
	}
	if user.Email != expectedUser.Email {
		t.Errorf("Got (%s) for email, expected (%s)", user.Email, expectedUser.Email)
	}
}

func TestUserCreateValidationError(t *testing.T) {
	cases := []struct {
		name string
		user *models.User
	}{
		{
			name: "No username",
			user: &models.User{
				Email:    "test@test.com",
				Password: "t3$t",
			},
		},
		{
			name: "No email",
			user: &models.User{
				Username: "test",
				Password: "t3$t",
			},
		},
		{
			name: "No password",
			user: &models.User{
				Username: "test",
				Email:    "test@test.com",
			},
		},
		{
			name: "Filled ID",
			user: &models.User{
				ID:       "1",
				Username: "test",
				Email:    "test@test.com",
				Password: "t3$t",
			},
		},
		{
			name: "Filled hashed password",
			user: &models.User{
				Username:       "test",
				Email:          "test@test.com",
				Password:       "t3$t",
				HashedPassword: []byte("t3$t"),
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := setupUsersServer(nil, nil)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/users", fixtures.Marshall(t, c.user))
			srv.ServeHTTP(w, r)

			got, expected := w.Code, http.StatusBadRequest
			if got != expected {
				t.Fatalf("Got (%d) for status code, expected (%d)", got, expected)
			}
		})
	}
}

func TestUserCreateDBError(t *testing.T) {
	cases := []struct {
		name         string
		dbError      error
		expectedCode int
	}{
		{
			name:         "Email already exists",
			dbError:      postgres.ErrEmailAlreadyExists,
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "Name already exists",
			dbError:      postgres.ErrUsernameAlreadyExists,
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "Unknown error",
			dbError:      errors.New("unknown error"),
			expectedCode: http.StatusInternalServerError,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			userModel := fixtures.NewUserModelMock(ctrl)

			srv := setupUsersServer(userModel, nil)

			userModel.EXPECT().
				Insert(&expectedUser).
				Return(nil, c.dbError)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/users", fixtures.Marshall(t, expectedUser))
			srv.ServeHTTP(w, r)

			got, expected := w.Code, c.expectedCode
			if got != expected {
				t.Fatalf("Got (%d) for status code, expected (%d)", got, expected)
			}
		})
	}
}

func TestUserLogin(t *testing.T) {
	ctrl := gomock.NewController(t)

	userModel := fixtures.NewUserModelMock(ctrl)
	authenticatorMock := fixtures.NewAuthenticatorMock(ctrl)

	srv := setupUsersServer(userModel, authenticatorMock)

	userModel.EXPECT().
		Authenticate(expectedUser.Email, expectedUser.Password).
		Return(&expectedUser, nil)

	token := "my_test_token"
	authenticatorMock.EXPECT().
		NewTokenForUser(&expectedUser).
		Return(token, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/users/login", fixtures.Marshall(t, expectedUser))
	srv.ServeHTTP(w, r)

	got, expected := w.Code, http.StatusOK
	if got != expected {
		t.Fatalf("Got (%d) for status code, expected (%d)", got, expected)
	}
	var userResponse models.UserResponse
	fixtures.Decode(t, w.Body, &userResponse)
	if userResponse.Token != token {
		t.Fatalf(`Got ("%s") for token, expected ("%s")`, userResponse.Token, token)
	}
}

func TestUserLoginValidationError(t *testing.T) {
	testCases := []struct {
		name string
		user *models.User
	}{
		{
			name: "No email",
			user: &models.User{
				Email:    "",
				Password: "T3$T",
			},
		},
		{
			name: "No password",
			user: &models.User{
				Email:    "test@mail.com",
				Password: "",
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			userModel := fixtures.NewUserModelMock(ctrl)

			srv := setupUsersServer(userModel, nil)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/users/login", fixtures.Marshall(t, testCase.user))
			srv.ServeHTTP(w, r)

			got, expected := w.Code, http.StatusBadRequest
			if got != expected {
				t.Fatalf("Got (%d) for status code, expected (%d)", got, expected)
			}
			// TODO: assert body, once we start returning proper errors
		})
	}
}

func TestUserLoginServiceError(t *testing.T) {
	testCases := []struct {
		name  string
		setup func(u *fixtures.UserModelMock, a *fixtures.AuthenticatorMock)
	}{
		{
			name: "UserModel.Authenticate fails",
			setup: func(u *fixtures.UserModelMock, a *fixtures.AuthenticatorMock) {
				u.EXPECT().
					Authenticate(expectedUser.Email, expectedUser.Password).
					Return(nil, errors.New("user not found"))
			},
		},
		{
			name: "Authenticator.NewTokenForUser fails",
			setup: func(u *fixtures.UserModelMock, a *fixtures.AuthenticatorMock) {
				u.EXPECT().
					Authenticate(expectedUser.Email, expectedUser.Password).
					Return(&expectedUser, nil)

				a.EXPECT().
					NewTokenForUser(&expectedUser).
					Return("", errors.New("intentional error"))
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			userModel := fixtures.NewUserModelMock(ctrl)
			authenticatorMock := fixtures.NewAuthenticatorMock(ctrl)

			testCase.setup(userModel, authenticatorMock)

			srv := setupUsersServer(userModel, authenticatorMock)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/users/login", fixtures.Marshall(t, expectedUser))
			srv.ServeHTTP(w, r)

			got, expected := w.Code, http.StatusInternalServerError
			if got != expected {
				t.Fatalf("Got (%d) for status code, expected (%d)", got, expected)
			}
		})
	}
}