package uic

import (
	"github.com/Cepave/fe/g"
	"github.com/Cepave/fe/http/base"
	. "github.com/Cepave/fe/model/uic"
	"github.com/Cepave/fe/utils"
	"github.com/toolkits/str"
	"strings"
	"time"
)

type AuthController struct {
	base.BaseController
}

func (this *AuthController) Logout() {
	u := this.Ctx.Input.GetData("CurrentUser").(*User)
	RemoveSessionByUid(u.Id)
	this.Ctx.SetCookie("sig", "", 0, "/")
	this.Redirect("/auth/login", 302)
}

func (this *AuthController) LoginGet() {
	appSig := this.GetString("sig", "")
	callback := this.GetString("callback", "")

	cookieSig := this.Ctx.GetCookie("sig")
	if cookieSig == "" {
		this.renderLoginPage(appSig, callback)
		return
	}

	sessionObj := ReadSessionBySig(cookieSig)
	if sessionObj == nil {
		this.renderLoginPage(appSig, callback)
		return
	}

        if int64(sessionObj.Expired) < time.Now().Unix() {
                RemoveSessionByUid(sessionObj.Uid)
                this.renderLoginPage(appSig, callback)
                return
        }

	if appSig != "" && callback != "" {
		this.Redirect(callback, 302)
	} else {
		this.Redirect("/me/info", 302)
	}
}

func (this *AuthController) LoginPost() {
	name := this.GetString("name", "")
	password := this.GetString("password", "")

	if name == "" || password == "" {
		this.ServeErrJson("name or password is blank")
		return
	}

	var u *User

	ldapEnabled := this.MustGetBool("ldap", false)

	if ldapEnabled {
		sucess, err := utils.LdapBind(g.Config().Ldap.Addr, name, password)
		if err != nil {
			this.ServeErrJson(err.Error())
			return
		}

		if !sucess {
			this.ServeErrJson("name or password error")
			return
		}

		arr := strings.Split(name, "@")
		var userName, userEmail string
		if len(arr) == 2 {
			userName = arr[0]
			userEmail = name
		} else {
			userName = name
			userEmail = ""
		}

		u = ReadUserByName(userName)
		if u == nil {
			// 说明用户不存在
			u = &User{
				Name:   userName,
				Passwd: "",
				Email:  userEmail,
			}
			_, err = u.Save()
			if err != nil {
				this.ServeErrJson("insert user fail " + err.Error())
				return
			}
		}
	} else {
		u = ReadUserByName(name)
		if u == nil {
			this.ServeErrJson("no such user")
			return
		}

		if u.Passwd != str.Md5Encode(g.Config().Salt+password) {
			this.ServeErrJson("password error")
			return
		}
	}

	appSig := this.GetString("sig", "")
	callback := this.GetString("callback", "")
	if appSig != "" && callback != "" {
		SaveSessionAttrs(u.Id, appSig, int(time.Now().Unix()) + 3600*24*30)
	} else {
		this.CreateSession(u.Id, 3600*24*30)
	}

	this.ServeDataJson(callback)
}

func (this *AuthController) renderLoginPage(sig, callback string) {
	this.Data["CanRegister"] = g.Config().CanRegister
	this.Data["LdapEnabled"] = g.Config().Ldap.Enabled
	this.Data["Sig"] = sig
	this.Data["Callback"] = callback
	this.TplNames = "auth/login.html"
}

func (this *AuthController) RegisterGet() {
	this.Data["CanRegister"] = g.Config().CanRegister
	this.TplNames = "auth/register.html"
}

func (this *AuthController) RegisterPost() {
	if !g.Config().CanRegister {
		this.ServeErrJson("registration system is not open")
		return
	}

	name := strings.TrimSpace(this.GetString("name", ""))
	password := strings.TrimSpace(this.GetString("password", ""))
	repeatPassword := strings.TrimSpace(this.GetString("repeat_password", ""))

	if password != repeatPassword {
		this.ServeErrJson("password not equal the repeart one")
		return
	}

	if !utils.IsUsernameValid(name) {
		this.ServeErrJson("name pattern is invalid")
		return
	}

	if ReadUserIdByName(name) > 0 {
		this.ServeErrJson("name is already existent")
		return
	}

	lastId, err := InsertRegisterUser(name, str.Md5Encode(g.Config().Salt+password))
	if err != nil {
		this.ServeErrJson("insert user fail " + err.Error())
		return
	}

	this.CreateSession(lastId, 3600*24*30)

	this.ServeOKJson()
}

func (this *AuthController) CreateSession(uid int64, maxAge int) int {
	sig := utils.GenerateUUID()
	expired := int(time.Now().Unix()) + maxAge
	SaveSessionAttrs(uid, sig, expired)
	this.Ctx.SetCookie("sig", sig, maxAge, "/")
	return expired
}
