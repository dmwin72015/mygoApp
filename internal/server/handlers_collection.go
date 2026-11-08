package server

import (
	"errors"
	"net/http"
	"reflect"
	"strconv"

	"mygo-app/internal/domain"
	"mygo-app/internal/i18n"

	"github.com/gin-gonic/gin"
)

// BindIdentity sets the identity of item from rawID. If item implements domain.Identifiable,
// it uses direct method dispatch (zero reflection, highest performance).
// As a fallback, reflection is used to safely populate ID or Code fields on any struct.
func BindIdentity[T any](item *T, rawID string) {
	if item == nil {
		return
	}
	if idObj, ok := any(item).(domain.Identifiable); ok {
		idObj.SetIdentity(rawID)
		return
	}
	// Fallback reflection for untyped or third-party structs
	val := reflect.ValueOf(item)
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return
	}
	if num, err := strconv.ParseInt(rawID, 10, 64); err == nil && num > 0 {
		if f := val.FieldByName("ID"); f.IsValid() && f.CanSet() && f.Kind() == reflect.Int64 {
			f.SetInt(num)
			return
		}
	}
	if f := val.FieldByName("Code"); f.IsValid() && f.CanSet() && f.Kind() == reflect.String {
		f.SetString(rawID)
	}
}

// registerCollectionRoutes registers REST endpoints on a RouterGroup for a collection Store[T].
// setID is optional; when omitted, BindIdentity is used by default.
func registerCollectionRoutes[T any](
	rg *gin.RouterGroup,
	path string,
	perm domain.Permission,
	store domain.Store[T],
	setID ...func(*T, string),
) {
	// List: GET /collection?search=&limit=
	rg.GET(path, RequirePermission(perm, false), func(c *gin.Context) {
		q := domain.Query{
			Search: c.Query("search"),
		}
		if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
			q.Limit = l
		}
		rows, err := store.List(c.Request.Context(), q)
		if err != nil {
			writeJSONError(c, http.StatusInternalServerError, err.Error())
			return
		}
		if rows == nil {
			rows = []T{}
		}
		c.JSON(http.StatusOK, gin.H{"items": rows})
	})

	// Get: GET /collection/:id
	rg.GET(path+"/:id", RequirePermission(perm, false), func(c *gin.Context) {
		lang := LangFromContext(c)
		id := c.Param("id")
		row, err := store.Get(c.Request.Context(), id)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				writeJSONError(c, http.StatusNotFound, i18n.Tr(lang, "err.not_found"))
				return
			}
			writeJSONError(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, row)
	})

	// Create: POST /collection
	rg.POST(path, RequirePermission(perm, true), func(c *gin.Context) {
		lang := LangFromContext(c)
		var item T
		if err := c.ShouldBindJSON(&item); err != nil {
			writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.invalid_req"))
			return
		}
		if err := store.Save(c.Request.Context(), &item); err != nil {
			writeJSONError(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, item)
	})

	// Update: PUT /collection/:id
	rg.PUT(path+"/:id", RequirePermission(perm, true), func(c *gin.Context) {
		lang := LangFromContext(c)
		id := c.Param("id")
		var item T
		if err := c.ShouldBindJSON(&item); err != nil {
			writeJSONError(c, http.StatusBadRequest, i18n.Tr(lang, "err.invalid_req"))
			return
		}
		if len(setID) > 0 && setID[0] != nil {
			setID[0](&item, id)
		} else {
			BindIdentity(&item, id)
		}
		if err := store.Save(c.Request.Context(), &item); err != nil {
			writeJSONError(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, item)
	})

	// Delete: DELETE /collection/:id
	rg.DELETE(path+"/:id", RequirePermission(perm, true), func(c *gin.Context) {
		id := c.Param("id")
		if err := store.Delete(c.Request.Context(), id); err != nil {
			writeJSONError(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
}
