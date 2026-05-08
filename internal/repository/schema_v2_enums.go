package repository

import "github.com/wujunhui99/agents_im/internal/model"

const (
	accountTypeDBAdmin int16 = 0
	accountTypeDBUser  int16 = 1
	accountTypeDBAgent int16 = 2

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

	mediaStorageProviderDBS3   int16 = 1
	mediaPurposeDBAvatar       int16 = 1
	mediaPurposeDBMessageImage int16 = 2
	mediaPurposeDBMessageFile  int16 = 3
	mediaPurposeDBAgentSkill   int16 = 4
	mediaStatusDBPending       int16 = 1
	mediaStatusDBReady         int16 = 2
	mediaStatusDBRejected      int16 = 3
	mediaStatusDBDeleted       int16 = 4
)

func accountTypeToDB(t model.AccountType) int16 {
	switch t {
	case model.AccountTypeAdmin:
		return accountTypeDBAdmin
	case model.AccountTypeAgent:
		return accountTypeDBAgent
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

func memberRoleToDB(role string) int16 {
	switch role {
	case model.MemberRoleOwner:
		return groupMemberRoleDBOwner
	case model.MemberRoleAdmin:
		return groupMemberRoleDBAdmin
	default:
		return groupMemberRoleDBMember
	}
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

func mediaPurposeToDB(p model.MediaPurpose) int16 {
	switch p {
	case model.MediaPurposeAvatar:
		return mediaPurposeDBAvatar
	case model.MediaPurposeMessageImage:
		return mediaPurposeDBMessageImage
	case model.MediaPurposeMessageFile:
		return mediaPurposeDBMessageFile
	case model.MediaPurposeAgentSkill:
		return mediaPurposeDBAgentSkill
	default:
		return mediaPurposeDBMessageFile
	}
}

func mediaPurposeFromDB(v int16) model.MediaPurpose {
	switch v {
	case mediaPurposeDBAvatar:
		return model.MediaPurposeAvatar
	case mediaPurposeDBMessageImage:
		return model.MediaPurposeMessageImage
	case mediaPurposeDBAgentSkill:
		return model.MediaPurposeAgentSkill
	default:
		return model.MediaPurposeMessageFile
	}
}

func mediaStatusToDB(s model.MediaStatus) int16 {
	switch s {
	case model.MediaStatusReady:
		return mediaStatusDBReady
	case model.MediaStatusRejected:
		return mediaStatusDBRejected
	case model.MediaStatusDeleted:
		return mediaStatusDBDeleted
	default:
		return mediaStatusDBPending
	}
}

func mediaStatusFromDB(v int16) model.MediaStatus {
	switch v {
	case mediaStatusDBReady:
		return model.MediaStatusReady
	case mediaStatusDBRejected:
		return model.MediaStatusRejected
	case mediaStatusDBDeleted:
		return model.MediaStatusDeleted
	default:
		return model.MediaStatusPending
	}
}
