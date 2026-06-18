package logic

import (
	"database/sql"
	"errors"
	"net/url"
	"strings"
	"time"

	sharemodel "github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/model"
)

// parseEmailVerifiedAt 解析 wire 上的 RFC3339 email_verified_at（空串=未验证→NULL）。
// auth 注册校验邮箱后经此带入，落 accounts.email_verified_at；契约保持 string（ADR #529 同规）。
func parseEmailVerifiedAt(value string) (sql.NullTime, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return sql.NullTime{}, nil
	}
	t, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return sql.NullTime{}, apperror.InvalidArgument("email_verified_at must be RFC3339 or empty")
	}
	return sql.NullTime{Time: t.UTC(), Valid: true}, nil
}

// facetForAccountType maps the granular DB account_type to the D16 account-facet
// encoded in the account id. Only agent accounts get FacetAgent (toPush=0);
// every other type (user/admin/test) is human-side (toPush=1). This keeps the id
// facet bit in lockstep with accounts.account_type — the D16 double-source
// invariant asserted at the creation boundary.
func facetForAccountType(accountTypeDB int64) idgen.Facet {
	if accountTypeDB == model.AccountTypeAgent {
		return idgen.FacetAgent
	}
	return idgen.FacetHuman
}

// gender 字符串取值（transport 层）。
const (
	genderUnknown = "unknown"
	genderMale    = "male"
	genderFemale  = "female"
	genderOther   = "other"
)

// validateIdentifier 校验并规范化 identifier。
// 注意：小写化 + 去空白是 identifier 这个唯一键的载荷性规范化（DB 内为小写、查找大小写不敏感），
// 必须服务端保证，故保留（不属于客户端可代劳的 cosmetic 规范化）。
func validateIdentifier(identifier string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(identifier))
	if len(normalized) < 3 || len(normalized) > 32 {
		return "", apperror.InvalidArgument("identifier must be 3 to 32 characters")
	}
	for idx, r := range normalized {
		isLetter := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		isUnderscore := r == '_'
		if idx == 0 && !isLetter && !isDigit {
			return "", apperror.InvalidArgument("identifier must start with a letter or digit")
		}
		if !isLetter && !isDigit && !isUnderscore {
			return "", apperror.InvalidArgument("identifier can only contain letters, digits, and underscore")
		}
	}
	return normalized, nil
}

// resolveNames 处理 display_name/name 的业务回填规则（二者缺省互补、全空回落 identifier），
// 再各自校验非空 + 长度上限。回填/互补是业务规则（非 cosmetic 规范化），保留。
func resolveNames(displayName, name, fallback string) (string, string, error) {
	displayName = strings.TrimSpace(displayName)
	name = strings.TrimSpace(name)
	if displayName == "" && name == "" {
		displayName = fallback
		name = fallback
	}
	if displayName == "" {
		displayName = name
	}
	if name == "" {
		name = displayName
	}
	displayName, err := validateProfileName(displayName, "display_name")
	if err != nil {
		return "", "", err
	}
	name, err = validateProfileName(name, "name")
	if err != nil {
		return "", "", err
	}
	return displayName, name, nil
}

func validateProfileName(value, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperror.InvalidArgument(field + " cannot be empty")
	}
	if len([]rune(value)) > 64 {
		return "", apperror.InvalidArgument(field + " must be 64 characters or fewer")
	}
	return value, nil
}

func validateRegion(region string) (string, error) {
	region = strings.TrimSpace(region)
	if len([]rune(region)) > 128 {
		return "", apperror.InvalidArgument("region must be 128 characters or fewer")
	}
	return region, nil
}

// validateGender 校验 gender 字符串（空 → unknown），返回规范化字符串。
func validateGender(gender string) (string, error) {
	gender = strings.ToLower(strings.TrimSpace(gender))
	if gender == "" {
		return genderUnknown, nil
	}
	switch gender {
	case genderUnknown, genderMale, genderFemale, genderOther:
		return gender, nil
	default:
		return "", apperror.InvalidArgument("gender must be unknown, male, female, or other")
	}
}

// accountTypeToDB 校验 account_type 字符串并映射为 DB 整型。
func accountTypeToDB(accountType string) (int64, error) {
	normalized, ok := sharemodel.NormalizeAccountType(accountType)
	if !ok {
		return 0, apperror.InvalidArgument("account_type must be user, agent, admin, or test")
	}
	switch normalized {
	case sharemodel.AccountTypeAdmin:
		return model.AccountTypeAdmin, nil
	case sharemodel.AccountTypeAgent:
		return model.AccountTypeAgent, nil
	case sharemodel.AccountTypeTest:
		return model.AccountTypeTest, nil
	default:
		return model.AccountTypeUser, nil
	}
}

func accountTypeFromDB(v int64) string {
	switch v {
	case model.AccountTypeAdmin:
		return string(sharemodel.AccountTypeAdmin)
	case model.AccountTypeAgent:
		return string(sharemodel.AccountTypeAgent)
	case model.AccountTypeTest:
		return string(sharemodel.AccountTypeTest)
	default:
		return string(sharemodel.AccountTypeUser)
	}
}

func genderToDB(gender string) int64 {
	switch gender {
	case genderMale:
		return model.GenderMale
	case genderFemale:
		return model.GenderFemale
	case genderOther:
		return model.GenderOther
	default:
		return model.GenderUnknown
	}
}

func genderFromDB(v int64) string {
	switch v {
	case model.GenderMale:
		return genderMale
	case model.GenderFemale:
		return genderFemale
	case model.GenderOther:
		return genderOther
	default:
		return genderUnknown
	}
}

// DurableAvatarURL 由 media id 推导稳定的头像访问路径（与 monolith 行为一致）。
func DurableAvatarURL(mediaID string) string {
	mediaID = strings.TrimSpace(mediaID)
	if mediaID == "" {
		return ""
	}
	return "/media/avatars/" + url.PathEscape(mediaID)
}

// mapAccountWriteError 把写入类 DB 错误映射为统一 apperror（唯一冲突→AlreadyExists，check→InvalidArgument）。
func mapAccountWriteError(err error) error {
	switch {
	case model.IsUniqueViolation(err):
		return apperror.AlreadyExists("identifier already exists")
	case model.IsCheckViolation(err):
		return apperror.InvalidArgument("invalid account profile or account_type")
	default:
		return err
	}
}

// mapReadError 把读取类 ErrNotFound 映射为 apperror.NotFound。
func mapReadError(err error) error {
	if errors.Is(err, model.ErrNotFound) {
		return apperror.NotFound("account not found")
	}
	return err
}
