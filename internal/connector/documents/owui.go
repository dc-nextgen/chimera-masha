package documents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// OWUIClient — Open WebUI'nin knowledge/files REST uclarina minimal bir istemci.
//
// DURUSTLUK: bu rotalar E7/RAG calismasindan (docs/panel-durum.md §F) BEST-EFFORT tasindi;
// OWUI SURUM-BAGIMLIDIR (biz OWUI v0.9.4'e PINLENDIK — bkz. CLAUDE.md §"Yönetim paneli").
// Gercek bir OWUI ornegine karsi CANLI DOGRULAMA, bu connector main.go'ya baglanirken (bir
// sonraki adim) yapilmali; burada YALNIZ hermetik httptest mocklariyla test edildi.
type OWUIClient struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func (c *OWUIClient) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *OWUIClient) authHeader(req *http.Request) {
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
}

// idResponse — OWUI'nin dondurdugu birkac olasi sekli karsilamak icin gevsek bir sarmalayici.
type idResponse struct {
	ID string `json:"id"`
}

// UploadFile — POST {BaseURL}/api/v1/files (multipart, alan adi "file"). Basarili yanitta
// dosya id'sini dondurur.
func (c *OWUIClient) UploadFile(ctx context.Context, filename string, content []byte) (string, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(content); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/v1/files", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	c.authHeader(req)

	resp, err := c.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("documents: owui UploadFile %d: %s", resp.StatusCode, string(respBody))
	}

	id, err := parseFileID(respBody)
	if err != nil {
		return "", fmt.Errorf("documents: owui UploadFile yanit ayrıştırılamadı: %w", err)
	}
	return id, nil
}

// parseFileID — OWUI genelde {"id": "..."} doner; bazi surumlerde/uclarda {"file": {"id": "..."}}
// gibi sarmalanmis olabilir. Ikisini de dener (savunmaci — bkz. paket yorumu, surum-bagimli).
func parseFileID(body []byte) (string, error) {
	var flat idResponse
	if err := json.Unmarshal(body, &flat); err == nil && flat.ID != "" {
		return flat.ID, nil
	}
	var wrapped struct {
		File idResponse `json:"file"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil && wrapped.File.ID != "" {
		return wrapped.File.ID, nil
	}
	return "", fmt.Errorf("id alani bulunamadi: %s", string(body))
}

func (c *OWUIClient) postJSON(ctx context.Context, path string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.authHeader(req)

	resp, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("documents: owui POST %s %d: %s", path, resp.StatusCode, string(respBody))
	}
	return nil
}

// AddFileToKnowledge — POST {BaseURL}/api/v1/knowledge/{id}/file/add {"file_id": fileID}.
func (c *OWUIClient) AddFileToKnowledge(ctx context.Context, knowledgeID, fileID string) error {
	return c.postJSON(ctx, "/api/v1/knowledge/"+knowledgeID+"/file/add", map[string]string{"file_id": fileID})
}

// RemoveFileFromKnowledge — POST {BaseURL}/api/v1/knowledge/{id}/file/remove {"file_id": fileID}.
func (c *OWUIClient) RemoveFileFromKnowledge(ctx context.Context, knowledgeID, fileID string) error {
	return c.postJSON(ctx, "/api/v1/knowledge/"+knowledgeID+"/file/remove", map[string]string{"file_id": fileID})
}

// DeleteFile — DELETE {BaseURL}/api/v1/files/{id}.
func (c *OWUIClient) DeleteFile(ctx context.Context, fileID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.BaseURL+"/api/v1/files/"+fileID, nil)
	if err != nil {
		return err
	}
	c.authHeader(req)

	resp, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("documents: owui DeleteFile %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
