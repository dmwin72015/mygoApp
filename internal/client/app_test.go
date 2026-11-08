package client

import (
	"runtime"
	"testing"

	"mygo-app/internal/i18n"

	"github.com/egoist/mygo"
)

func TestBuildAppMenu(t *testing.T) {
	i18n.SetLanguage(i18n.LangZH)
	menu := buildAppMenu(nil)
	items := menu.Items()

	if len(items) == 0 {
		t.Fatal("expected menu items, got 0")
	}

	// 1. First top-level menu must be RoleAppMenu
	firstMenu := items[0]
	if firstMenu.Role != mygo.RoleAppMenu {
		t.Errorf("expected first top-level menu to have RoleAppMenu, got %s", firstMenu.Role)
	}

	// 2. RoleAppMenu submenu must contain "设置" (settings.title)
	foundSettingsInAppMenu := false
	for _, it := range firstMenu.Submenu {
		if it.Label == "设置" && it.Accelerator == "CmdOrCtrl+," {
			foundSettingsInAppMenu = true
			break
		}
	}
	if !foundSettingsInAppMenu {
		t.Errorf("expected '设置' inside RoleAppMenu (myGo App Dev) submenu")
	}

	// 3. Check "文件" menu behavior by platform
	if len(items) > 1 {
		fileMenu := items[1]
		if fileMenu.Label != "文件" {
			t.Errorf("expected second menu to be '文件', got %s", fileMenu.Label)
		}

		foundInFileMenu := false
		for _, it := range fileMenu.Submenu {
			if it.Label == "设置" {
				foundInFileMenu = true
				break
			}
		}

		if runtime.GOOS == "darwin" && foundInFileMenu {
			t.Errorf("on darwin, '设置' should only be in RoleAppMenu (first group), not in '文件'")
		}
		if runtime.GOOS != "darwin" && !foundInFileMenu {
			t.Errorf("on non-darwin, '设置' should be in '文件' menu")
		}
	}
}
