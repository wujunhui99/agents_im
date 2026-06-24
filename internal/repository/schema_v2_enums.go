package repository

import "github.com/wujunhui99/agents_im/pkg/model"

const (
	accountTypeDBAdmin int16 = 0
	accountTypeDBUser  int16 = 1
	accountTypeDBAgent int16 = 2
	accountTypeDBTest  int16 = 3

	genderDBUnknown int16 = 0
	genderDBMale    int16 = 1
	genderDBFemale  int16 = 2
	genderDBOther   int16 = 3

	friendshipStatusDBPending  int16 = 1
	friendshipStatusDBAccepted int16 = 2
	friendshipStatusDBRejected int16 = 3
	friendshipStatusDBDeleted  int16 = 4

	groupMemberRoleDBOwner    int16 = 1
	groupMemberRoleDBMember   int16 = 2
	groupMemberRoleDBAdmin    int16 = 3
	groupMemberStatusDBActive int16 = 1
	groupMemberStatusDBLeft   int16 = 2
)

func accountTypeToDB(t model.AccountType) int16 {
	switch t {
	case model.AccountTypeAdmin:
		return accountTypeDBAdmin
	case model.AccountTypeAgent:
		return accountTypeDBAgent
	case model.AccountTypeTest:
		return accountTypeDBTest
	default:
		return accountTypeDBUser
	}
}

func accountTypeFromDB(v int16) model.AccountType {
	switch v {
	case accountTypeDBAdmin:
		return model.AccountTypeAdmin
	case accountTypeDBAgent:
		return model.AccountTypeAgent
	case accountTypeDBTest:
		return model.AccountTypeTest
	default:
		return model.AccountTypeUser
	}
}

func genderToDB(g string) int16 {
	switch g {
	case "male":
		return genderDBMale
	case "female":
		return genderDBFemale
	case "other":
		return genderDBOther
	default:
		return genderDBUnknown
	}
}

func genderFromDB(v int16) string {
	switch v {
	case genderDBMale:
		return "male"
	case genderDBFemale:
		return "female"
	case genderDBOther:
		return "other"
	default:
		return "unknown"
	}
}

func friendshipStatusToDB(status string) int16 {
	switch status {
	case model.FriendshipStatusAccepted:
		return friendshipStatusDBAccepted
	case model.FriendshipStatusRejected:
		return friendshipStatusDBRejected
	case model.FriendshipStatusDeleted:
		return friendshipStatusDBDeleted
	default:
		return friendshipStatusDBPending
	}
}

func friendshipStatusFromDB(v int16) string {
	switch v {
	case friendshipStatusDBAccepted:
		return model.FriendshipStatusAccepted
	case friendshipStatusDBRejected:
		return model.FriendshipStatusRejected
	case friendshipStatusDBDeleted:
		return model.FriendshipStatusDeleted
	default:
		return model.FriendshipStatusPending
	}
}

func memberStateToDB(state string) int16 {
	if state == model.MemberStateLeft {
		return groupMemberStatusDBLeft
	}
	return groupMemberStatusDBActive
}

func memberStateFromDB(v int16) string {
	if v == groupMemberStatusDBLeft {
		return model.MemberStateLeft
	}
	return model.MemberStateActive
}

func memberRoleFromDB(v int16) string {
	switch v {
	case groupMemberRoleDBOwner:
		return model.MemberRoleOwner
	case groupMemberRoleDBAdmin:
		return model.MemberRoleAdmin
	default:
		return model.MemberRoleMember
	}
}
