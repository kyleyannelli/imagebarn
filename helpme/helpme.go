package helpme

import (
	"sync"
)

func NewApprovedUsers() *ApprovedUsers {
	return &ApprovedUsers{users: make(map[string]bool), rwMutex: sync.RWMutex{}}
}

func (au *ApprovedUsers) Disapprove(email string) {
	au.rwMutex.Lock()
	au.users[email] = false
	au.rwMutex.Unlock()
}

func (au *ApprovedUsers) Approve(email string) {
	au.rwMutex.Lock()
	au.users[email] = true
	au.rwMutex.Unlock()
}

func (au *ApprovedUsers) IsApproved(email string) bool {
	au.rwMutex.RLock()
	isApproved, exists := au.users[email]
	if !exists {
		au.rwMutex.RUnlock()
		au.rwMutex.Lock()
		au.users[email] = false
		au.rwMutex.Unlock()
	} else {
		au.rwMutex.RUnlock()
	}
	return isApproved
}

// use this for operations where we dont need the entire map
func (au *ApprovedUsers) SetSizeCopyOfUsersMap(count int) map[string]bool {
	copiedMap := map[string]bool{}
	au.rwMutex.RLock()
	i := 0
	for email, isApproved := range au.users {
		copiedMap[email] = isApproved
		i++
		if i >= count {
			break
		}
	}
	au.rwMutex.RUnlock()
	return copiedMap
}

func (au *ApprovedUsers) CopyOfUsersMap() map[string]bool {
	copiedMap := map[string]bool{}
	au.rwMutex.RLock()
	for email, isApproved := range au.users {
		copiedMap[email] = isApproved
	}
	au.rwMutex.RUnlock()
	return copiedMap
}
