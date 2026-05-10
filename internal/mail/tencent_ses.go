package mail

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	tencentSESAction  = "SendEmail"
	tencentSESService = "ses"
	tencentSESVersion = "2020-10-02"
)

type TencentSESProvider struct {
	cfg        TencentSESConfig
	httpClient *http.Client
	now        func() time.Time
}

func NewTencentSESProvider(cfg TencentSESConfig, httpClient *http.Client) (*TencentSESProvider, error) {
	cfg = cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if _, err := cfg.DefaultTemplateIDValue(); err != nil {
		return nil, err
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &TencentSESProvider{
		cfg:        cfg,
		httpClient: httpClient,
		now:        time.Now,
	}, nil
}

func (p *TencentSESProvider) SendTemplateEmail(ctx context.Context, req TemplateEmailRequest) (TemplateEmailResponse, error) {
	payload, err := p.buildPayload(req)
	if err != nil {
		return TemplateEmailResponse{}, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return TemplateEmailResponse{}, fmt.Errorf("marshal tencent ses request: %w", err)
	}

	endpoint, err := endpointURL(p.cfg.Endpoint)
	if err != nil {
		return TemplateEmailResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return TemplateEmailResponse{}, fmt.Errorf("build tencent ses request: %w", err)
	}
	timestamp := p.now().Unix()
	httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")
	httpReq.Header.Set("X-TC-Action", tencentSESAction)
	httpReq.Header.Set("X-TC-Version", tencentSESVersion)
	httpReq.Header.Set("X-TC-Region", p.cfg.Region)
	httpReq.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", timestamp))
	httpReq.Header.Set("Authorization", p.authorization(endpoint.Host, body, timestamp))

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return TemplateEmailResponse{}, fmt.Errorf("send tencent ses request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return TemplateEmailResponse{}, fmt.Errorf("read tencent ses response: %w", err)
	}
	envelope, err := decodeTencentSESResponse(respBody)
	if err != nil {
		return TemplateEmailResponse{}, err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return TemplateEmailResponse{}, &ProviderError{
			Provider:  ProviderTencentSES,
			Code:      fmt.Sprintf("HTTPStatus%d", httpResp.StatusCode),
			Message:   http.StatusText(httpResp.StatusCode),
			RequestID: envelope.Response.RequestID,
		}
	}
	if envelope.Response.Error != nil {
		return TemplateEmailResponse{}, &ProviderError{
			Provider:  ProviderTencentSES,
			Code:      envelope.Response.Error.Code,
			Message:   envelope.Response.Error.Message,
			RequestID: envelope.Response.RequestID,
		}
	}
	return TemplateEmailResponse{
		Provider:          ProviderTencentSES,
		ProviderRequestID: envelope.Response.RequestID,
		ProviderMessageID: envelope.Response.MessageID,
		Status:            SendStatusAccepted,
	}, nil
}

func (p *TencentSESProvider) buildPayload(req TemplateEmailRequest) (tencentSESSendEmailRequest, error) {
	templateID := req.TemplateID
	if templateID == 0 {
		var err error
		templateID, err = p.cfg.DefaultTemplateIDValue()
		if err != nil {
			return tencentSESSendEmailRequest{}, err
		}
	}
	if templateID == 0 {
		return tencentSESSendEmailRequest{}, fmt.Errorf("template id is required")
	}
	templateData, err := json.Marshal(req.TemplateData)
	if err != nil {
		return tencentSESSendEmailRequest{}, fmt.Errorf("marshal template data: %w", err)
	}
	fromEmail := strings.TrimSpace(req.FromEmail)
	if fromEmail == "" {
		fromEmail = p.cfg.FromEmailAddress
	}
	return tencentSESSendEmailRequest{
		FromEmailAddress: fromEmail,
		Destination:      append([]string(nil), req.Recipients...),
		Subject:          strings.TrimSpace(req.Subject),
		Template: tencentSESTemplate{
			TemplateID:   templateID,
			TemplateData: string(templateData),
		},
	}, nil
}

func (p *TencentSESProvider) authorization(host string, payload []byte, timestamp int64) string {
	hashedPayload := sha256Hex(payload)
	canonicalHeaders := fmt.Sprintf("content-type:application/json; charset=utf-8\nhost:%s\n", host)
	canonicalRequest := strings.Join([]string{
		http.MethodPost,
		"/",
		"",
		canonicalHeaders,
		"content-type;host",
		hashedPayload,
	}, "\n")

	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, tencentSESService)
	stringToSign := strings.Join([]string{
		"TC3-HMAC-SHA256",
		fmt.Sprintf("%d", timestamp),
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	secretDate := hmacSHA256([]byte("TC3"+p.cfg.SecretKey), date)
	secretService := hmacSHA256(secretDate, tencentSESService)
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))

	return fmt.Sprintf(
		"TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=content-type;host, Signature=%s",
		p.cfg.SecretID,
		credentialScope,
		signature,
	)
}

type tencentSESSendEmailRequest struct {
	FromEmailAddress string             `json:"FromEmailAddress"`
	Destination      []string           `json:"Destination"`
	Subject          string             `json:"Subject,omitempty"`
	Template         tencentSESTemplate `json:"Template"`
}

type tencentSESTemplate struct {
	TemplateID   uint64 `json:"TemplateID"`
	TemplateData string `json:"TemplateData"`
}

type tencentSESResponseEnvelope struct {
	Response struct {
		Error     *tencentSESResponseError `json:"Error,omitempty"`
		RequestID string                   `json:"RequestId"`
		MessageID string                   `json:"MessageId"`
	} `json:"Response"`
}

type tencentSESResponseError struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

func decodeTencentSESResponse(body []byte) (tencentSESResponseEnvelope, error) {
	var envelope tencentSESResponseEnvelope
	if len(bytes.TrimSpace(body)) == 0 {
		return envelope, &ProviderError{
			Provider: ProviderTencentSES,
			Code:     "EmptyResponse",
			Message:  "empty Tencent SES response",
		}
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return envelope, fmt.Errorf("decode tencent ses response: %w", err)
	}
	return envelope, nil
}

func endpointURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = defaultTencentSESEndpoint
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse tencent ses endpoint: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported tencent ses endpoint scheme: %s", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("tencent ses endpoint host is required")
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return u, nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(data))
	return mac.Sum(nil)
}
