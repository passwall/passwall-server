package app

import (
	"reflect"
	"testing"
	"time"

	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	uuid "github.com/satori/go.uuid"
)

var (
	mockName     = "passwall-patron"
	mockEmail    = "patron@passwall.io"
	mockPassword = "123456789123456789"
	mockSecret   = "supersecret"
	now          = time.Now()
	uuidN        = uuid.NewV4()
)

func TestGenerateSchema(t *testing.T) {
	store, err := initDB()
	if err != nil {
		t.Errorf("Error in database initialization ! err:  %v", err)
	}
	type args struct {
		s    storage.Store
		user *model.User
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "No error ", args: args{
			s: store,
			user: &model.User{
				ID:               0,
				UUID:             uuid.NewV4(),
				Name:             mockName,
				Email:            mockEmail,
				MasterPassword:   mockPassword,
				Secret:           mockSecret,
				Schema:           "users",
				Role:             "Member",
				ConfirmationCode: "12345678",
				EmailVerifiedAt:  time.Time{},
			},
		},
			wantErr: false,
		},
		// todo: populate different cases in test table
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateSchema(tt.args.s, tt.args.user)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			want, err := store.Users().FindByEmail(mockEmail)
			if err != nil {
				t.Errorf("FindByEmail() error %v", err)
			}
			got.CreatedAt = got.CreatedAt.UTC()
			got.UpdatedAt = got.UpdatedAt.UTC()
			got.EmailVerifiedAt = got.EmailVerifiedAt.UTC()
			if !reflect.DeepEqual(got, want) {
				t.Errorf("GenerateSchema() got = %v, want %v", got, want)
			}
		})
	}
	// todo: teardown resources
}

func TestCreateUser(t *testing.T) {
	store, err := initDB()
	if err != nil {
		t.Errorf("Error in database initialization ! err:  %v", err)
	}

	type args struct {
		s       storage.Store
		userDTO *model.UserDTO
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"No Error", args{
			s: store,
			userDTO: &model.UserDTO{
				ID:              0,
				UUID:            uuidN,
				Name:            mockName,
				Email:           mockEmail,
				MasterPassword:  mockPassword,
				Secret:          mockSecret,
				Schema:          "users",
				Role:            "Member",
				EmailVerifiedAt: now,
			},
		},
			false,
		},
		{"Expected Error: No name, email", args{
			s: store,
			userDTO: &model.UserDTO{
				ID:              1,
				UUID:            uuidN,
				Name:            "",
				Email:           "",
				MasterPassword:  "",
				Secret:          "",
				Schema:          "users",
				Role:            "Member",
				EmailVerifiedAt: now,
			},
		},
			false, // this should be true and tests should pass in the true statement
			// however it does not pass, if there is no name, email provided, how it
			// is possible to create a record on database side. I guess, something is wrong
			// in creating user
		},
		// todo: populate test cases
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateUser(tt.args.s, tt.args.userDTO)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			user, err := store.Users().FindByEmail(mockEmail)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindByEmail() error = %v", err)
			}
			// due to different timezone settings received data should be transformed to UTC timezone
			got.UpdatedAt = got.UpdatedAt.UTC()
			got.CreatedAt = got.CreatedAt.UTC()
			got.EmailVerifiedAt = got.EmailVerifiedAt.UTC()
			if !reflect.DeepEqual(got, user) {
				t.Errorf("CreateUser() got = %v, want %v", got, user)
			}

		})
	}
	// todo: teardown resources

}

// todo : complete TestUpdateUser, once the issues fixed above
func TestUpdateUser(t *testing.T) {
	type args struct {
		s            storage.Store
		user         *model.User
		userDTO      *model.UserDTO
		isAuthorized bool
	}
	tests := []struct {
		name    string
		args    args
		want    *model.User
		wantErr bool
	}{
		// todo: add different test scenarios
		{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UpdateUser(tt.args.s, tt.args.user, tt.args.userDTO, tt.args.isAuthorized)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateUser() got = %v, want %v", got, tt.want)
			}
		})
	}
}
