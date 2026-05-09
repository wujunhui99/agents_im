package mail

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultTencentSESEndpoint          = "https://ses.tencentcloudapi.com"
	defaultTencentSESRegion            = "ap-hongkong"
	defaultTencentSESFromEmailAddress  = "noreply@agenticim.xyz"
	defaultTencentSESDefaultTemplateID = "177952"
)

type TencentSESConfig struct {
	SecretID          string
	SecretKey         string
	Region            string `json:",default=ap-hongkong"`
	Endpoint          string `json:",default=https://ses.tencentcloudapi.com"`
	FromEmailAddress  string `json:",default=noreply@agenticim.xyz"`
	DefaultTemplateID string `json:",default=177952"`
}

func (c TencentSESConfig) WithDefaults() TencentSESConfig {
	c.SecretID = strings.TrimSpace(resolveEnvPlaceholder(c.SecretID))
	c.SecretKey = strings.TrimSpace(resolveEnvPlaceholder(c.SecretKey))
	c.Region = strings.TrimSpace(resolveEnvPlaceholder(c.Region))
	c.Endpoint = strings.TrimSpace(resolveEnvPlaceholder(c.Endpoint))
	c.FromEmailAddress = strings.TrimSpace(resolveEnvPlaceholder(c.FromEmailAddress))
	c.DefaultTemplateID = strings.TrimSpace(resolveEnvPlaceholder(c.DefaultTemplateID))
	if c.Region == "" {
		c.Region = defaultTencentSESRegion
	}
	if c.Endpoint == "" {
		c.Endpoint = defaultTencentSESEndpoint
	}
	if c.FromEmailAddress == "" {
		c.FromEmailAddress = defaultTencentSESFromEmailAddress
	}
	if c.DefaultTemplateID == "" {
		c.DefaultTemplateID = defaultTencentSESDefaultTemplateID
	}
	return c
}

func (c TencentSESConfig) Validate() error {
	c = c.WithDefaults()
	var missing []string
	if missingConfigValue(c.SecretID) {
		missing = append(missing, "TencentSES.SecretID")
	}
	if missingConfigValue(c.SecretKey) {
		missing = append(missing, "TencentSES.SecretKey")
	}
	if missingConfigValue(c.Region) {
		missing = append(missing, "TencentSES.Region")
	}
	if missingConfigValue(c.Endpoint) {
		missing = append(missing, "TencentSES.Endpoint")
	}
	if missingConfigValue(c.FromEmailAddress) {
		missing = append(missing, "TencentSES.FromEmailAddress")
	}
	if len(missing) > 0 {
		return fmt.Errorf("mail provider config missing required values: %s", strings.Join(missing, ", "))
	}
	if missingConfigValue(c.DefaultTemplateID) {
		return errors.New("mail provider config missing required value: TencentSES.DefaultTemplateID")
	}
	if _, err := c.DefaultTemplateIDValue(); err != nil {
		return err
	}
	return nil
}

func (c TencentSESConfig) DefaultTemplateIDValue() (uint64, error) {
	c = c.WithDefaults()
	value, err := strconv.ParseUint(c.DefaultTemplateID, 10, 64)
	if err != nil || value == 0 {
		return 0, fmt.Errorf("mail provider config has invalid TencentSES.DefaultTemplateID: %q", c.DefaultTemplateID)
	}
	return value, nil
}

func missingConfigValue(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		return true
	}
	if strings.HasPrefix(strings.ToLower(value), "replace-with-") {
		return true
	}
	return false
}

func resolveEnvPlaceholder(value string) string {
	value = strings.TrimSpace(value)
	if !(strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}")) {
		return value
	}
	key := strings.TrimSuffix(strings.TrimPrefix(value, "${"), "}")
	if key == "" {
		return value
	}
	if resolved, ok := os.LookupEnv(key); ok {
		return resolved
	}
	return value
}
