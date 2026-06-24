package controllers

import (
	"github.com/beego/beego/v2/server/web/context"
)

// AdminAuthFilter — barcha admin sahifalarini himoya qiladi
func AdminAuthFilter(ctx *context.Context) {
	path := ctx.Request.URL.Path

	// Login sahifasini chetlab o'tamiz
	if path == "/admin/login" {
		return
	}

	// Sessiyani tekshirish
	if ctx.Input.Session("admin_logged_in") == nil {
		ctx.Redirect(302, "/admin/login") // ← To'g'ri tartib: status, url
		return
	}

	// Role tekshirish
	role := ctx.Input.Session("admin_role")
	if role == nil {
		ctx.Redirect(302, "/admin/login")
		return
	}

	roleStr, ok := role.(string)
	if !ok || (roleStr != "superadmin" && roleStr != "admin") {
		ctx.Redirect(302, "/admin/login")
		return
	}
}
