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

	friendshipStatusDBActive  int16 = 1
	friendshipStatusDBDeleted int16 = 2

	groupMemberRoleDBOwner    int16 = 1
	groupMemberRoleDBMember   int16 = 2
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
	if status == model.FriendshipStatusDeleted {
		return friendshipStatusDBDeleted
	}
	return friendshipStatusDBActive
}

func friendshipStatusFromDB(v int16) string {
	if v == friendshipStatusDBDeleted {
		return model.FriendshipStatusDeleted
	}
	return model.FriendshipStatusActive
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
