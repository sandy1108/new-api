package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/clientaccess"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func clientAccessManagementError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"success": false, "code": code, "message": http.StatusText(status)})
}

func clientAccessCredentialID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		clientAccessManagementError(c, http.StatusBadRequest, "CLIENT_ACCESS_INVALID_ID")
		return 0, false
	}
	return id, true
}

// CreateClientAccessCredential 只在本次成功响应中返回完整 Token。
func CreateClientAccessCredential(c *gin.Context) {
	var input clientaccess.CreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		clientAccessManagementError(c, http.StatusBadRequest, "CLIENT_ACCESS_INVALID_INPUT")
		return
	}
	issued, err := clientaccess.NewStore(model.DB).CreateCredential(c.GetInt("id"), input)
	if err != nil {
		if errors.Is(err, clientaccess.ErrInvalidInput) || errors.Is(err, clientaccess.ErrInvalidGrant) {
			clientAccessManagementError(c, http.StatusBadRequest, "CLIENT_ACCESS_INVALID_INPUT")
		} else {
			clientAccessManagementError(c, http.StatusInternalServerError, "CLIENT_ACCESS_INTERNAL_ERROR")
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": issued})
}

func ListClientAccessCredentials(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if limit < 1 || limit > 100 || page < 1 {
		clientAccessManagementError(c, http.StatusBadRequest, "CLIENT_ACCESS_INVALID_PAGINATION")
		return
	}
	query := model.DB.Model(&model.ClientAccessCredential{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		clientAccessManagementError(c, http.StatusInternalServerError, "CLIENT_ACCESS_INTERNAL_ERROR")
		return
	}
	var credentials []model.ClientAccessCredential
	if err := query.Order("id DESC").Limit(limit).Offset((page - 1) * limit).Find(&credentials).Error; err != nil {
		clientAccessManagementError(c, http.StatusInternalServerError, "CLIENT_ACCESS_INTERNAL_ERROR")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"items": credentials, "total": total}})
}

// GetClientAccessCredential 返回授权编辑所需的当前快照，不暴露凭据哈希或渠道密钥。
func GetClientAccessCredential(c *gin.Context) {
	id, ok := clientAccessCredentialID(c)
	if !ok {
		return
	}
	var credential model.ClientAccessCredential
	if err := model.DB.First(&credential, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			clientAccessManagementError(c, http.StatusNotFound, "CLIENT_ACCESS_NOT_FOUND")
		} else {
			clientAccessManagementError(c, http.StatusInternalServerError, "CLIENT_ACCESS_INTERNAL_ERROR")
		}
		return
	}
	grants, err := clientaccess.NewStore(model.DB).GetGrants(id)
	if err != nil {
		clientAccessManagementError(c, http.StatusInternalServerError, "CLIENT_ACCESS_INTERNAL_ERROR")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"credential": credential, "grants": grants}})
}

func UpdateClientAccessGrants(c *gin.Context) {
	id, ok := clientAccessCredentialID(c)
	if !ok {
		return
	}
	var input struct {
		Version int64                     `json:"version"`
		Grants  []clientaccess.GrantInput `json:"grants"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.Version <= 0 {
		clientAccessManagementError(c, http.StatusBadRequest, "CLIENT_ACCESS_INVALID_INPUT")
		return
	}
	credential, err := clientaccess.NewStore(model.DB).UpdateGrants(id, input.Version, input.Grants)
	if err != nil {
		if errors.Is(err, clientaccess.ErrVersionConflict) {
			clientAccessManagementError(c, http.StatusConflict, "CLIENT_ACCESS_VERSION_CONFLICT")
		} else if errors.Is(err, clientaccess.ErrInvalidGrant) {
			clientAccessManagementError(c, http.StatusBadRequest, "CLIENT_ACCESS_INVALID_GRANT")
		} else {
			clientAccessManagementError(c, http.StatusInternalServerError, "CLIENT_ACCESS_INTERNAL_ERROR")
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": credential})
}

func GetClientAccessScopes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": clientaccess.ScopeCatalog()})
}

func GetClientAccessScopeResources(c *gin.Context) {
	resources, err := clientaccess.NewStore(model.DB).ListScopeResources(c.Param("scope"))
	if err != nil {
		if errors.Is(err, clientaccess.ErrScopeDenied) {
			clientAccessManagementError(c, http.StatusNotFound, "CLIENT_ACCESS_UNKNOWN_SCOPE")
		} else {
			clientAccessManagementError(c, http.StatusInternalServerError, "CLIENT_ACCESS_INTERNAL_ERROR")
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": resources})
}

func clientAccessExpectedVersion(c *gin.Context) (int64, bool) {
	var input struct {
		Version int64 `json:"version"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.Version <= 0 {
		clientAccessManagementError(c, http.StatusBadRequest, "CLIENT_ACCESS_INVALID_VERSION")
		return 0, false
	}
	return input.Version, true
}

func RotateClientAccessCredential(c *gin.Context) {
	id, ok := clientAccessCredentialID(c)
	if !ok {
		return
	}
	version, ok := clientAccessExpectedVersion(c)
	if !ok {
		return
	}
	issued, err := clientaccess.NewStore(model.DB).RotateCredential(id, version)
	if err != nil {
		clientAccessWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": issued})
}

func clientAccessWriteError(c *gin.Context, err error) {
	if errors.Is(err, clientaccess.ErrVersionConflict) {
		clientAccessManagementError(c, http.StatusConflict, "CLIENT_ACCESS_VERSION_CONFLICT")
		return
	}
	clientAccessManagementError(c, http.StatusInternalServerError, "CLIENT_ACCESS_INTERNAL_ERROR")
}

func setClientAccessCredentialStatus(c *gin.Context, status int) {
	id, ok := clientAccessCredentialID(c)
	if !ok {
		return
	}
	version, ok := clientAccessExpectedVersion(c)
	if !ok {
		return
	}
	credential, err := clientaccess.NewStore(model.DB).SetStatus(id, version, status)
	if err != nil {
		clientAccessWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": credential})
}

func DisableClientAccessCredential(c *gin.Context) {
	setClientAccessCredentialStatus(c, clientaccess.CredentialStatusDisabled)
}

func EnableClientAccessCredential(c *gin.Context) {
	setClientAccessCredentialStatus(c, clientaccess.CredentialStatusEnabled)
}

func RevokeClientAccessCredential(c *gin.Context) {
	setClientAccessCredentialStatus(c, clientaccess.CredentialStatusRevoked)
}
