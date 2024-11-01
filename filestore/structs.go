package filestore

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"kmfg.dev/imagebarn/v1/helpme"
)

const APPROVED_USERS_FILE = "./approved-users.json"

var wg *sync.WaitGroup

type Filestore struct {
	approvedUsers       *helpme.ApprovedUsers
	storeRoutineRunning bool
}

func NewFilestore(adminUserEmail string, waitGroup *sync.WaitGroup) *Filestore {
	approvedUsers, err := loadApprovedUsers(adminUserEmail)
	if err != nil {
		panic(err)
	}
	wg = waitGroup
	return &Filestore{approvedUsers, false}
}

func (fs *Filestore) ApprovedUsers() *helpme.ApprovedUsers {
	return fs.approvedUsers
}

func (fs *Filestore) IsApproved(authUser *helpme.AuthUser) bool {
	return fs.approvedUsers.IsApproved(authUser.Email())
}

func (fs *Filestore) StoreApprovedUsersRoutine(stopChan chan struct{}) {
	if fs.storeRoutineRunning {
		return
	}
	go func() {
		fs.storeRoutineRunning = true
		wg.Add(1)
		defer wg.Done()
		for {
			select {
			case <-stopChan:
				slog.Info("Safely stopping approved user storage routine.")
				return
			default:
				time.Sleep(2 * time.Second)
				// why copy data then save?
				//  on devices that use sd card storage this could be super slow to lock for an entire marshal
				approvedUsersMap := fs.approvedUsers.CopyOfUsersMap()

				data, err := json.Marshal(approvedUsersMap)
				if err != nil {
					slog.Warn(fmt.Sprintf("Failed to marshal authorized user for storge!: %v", err))
					continue
				}

				file, err := os.Create(APPROVED_USERS_FILE)
				if err != nil {
					slog.Warn(fmt.Sprintf("Failed to create file authorized user storge!: %v", err))
					continue
				}

				_, err = file.Write(data)
				file.Close()
				if err != nil {
					slog.Warn(fmt.Sprintf("Failed write to file authorized user storge!: %v", err))
					continue
				}
			}
		}
	}()
}

func loadApprovedUsers(adminUserEmail string) (*helpme.ApprovedUsers, error) {
	data, err := os.ReadFile(APPROVED_USERS_FILE)
	if err != nil {
		approvedUsers := helpme.NewApprovedUsers()
		approvedUsers.Approve(adminUserEmail)
		slog.Warn(fmt.Sprintf("Failed to read file %v. Loading empty approve with admin only: %v", APPROVED_USERS_FILE, err))
		return approvedUsers, nil
	}

	var approvedUserTmp map[string]bool
	if err = json.Unmarshal(data, &approvedUserTmp); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal: %v", err)
	}

	approvedUsers := helpme.NewApprovedUsers()
	approvedUserTmp[adminUserEmail] = true
	for email, isApproved := range approvedUserTmp {
		if isApproved {
			approvedUsers.Approve(email)
		} else {
			approvedUsers.Disapprove(email)
		}
	}
	return approvedUsers, nil
}
