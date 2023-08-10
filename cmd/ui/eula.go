/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package ui

import (
	"fmt"
	"strings"

	"github.com/jroimartin/gocui"
	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	log "github.com/sirupsen/logrus"
)

var Agreed = true
var Disagreed = false
var LicenseAgreed *bool
var Extract = false

// LicenseWindowType defines the struct to handle UI
type LicenseWindowType struct {
	// LayoutManager is a function that defines the elements in the ui
	LayoutManager func(g *gocui.Gui) error

	Scroll   func(v *gocui.View, dy int) error
	ScrollUp func(g *gocui.Gui, v *gocui.View) error

	ScrollDown func(g *gocui.Gui, v *gocui.View) error

	Agree func(g *gocui.Gui, v *gocui.View) error

	Disagree func(g *gocui.Gui, v *gocui.View) error

	Extract func(g *gocui.Gui, v *gocui.View) error

	Gui *gocui.Gui
}

// DisplayAndWaitForEULA prints out the license to the user through a UI
// and waits for user confirmation.
func DisplayAndWaitForEULA(licenseTitle, licenseContents string) (bool, error) {
	if Extract {
		return false, errs.ErrExtractEula
	}

	promptText := "License Agreement: [A]ccept [D]ecline [E]xtract"

	if !utils.IsTerminalInteractive() {
		// Show input on non-interactive terminals
		promptText = "License Agreement: [A]ccept [D]ecline [E]xtract: "
		fmt.Printf("*** %v ***", licenseTitle)
		fmt.Println()
		fmt.Println(licenseContents)
		fmt.Println()
		fmt.Print(promptText)

		if LicenseAgreed != nil {
			return *LicenseAgreed, nil
		}

		if Extract {
			return false, errs.ErrExtractEula
		}

		var input string
		fmt.Scanln(&input)

		if input == "a" || input == "A" {
			return true, nil
		}

		if input == "e" || input == "E" {
			return false, errs.ErrExtractEula
		}

		return false, nil
	}

	licenseWindow := NewLicenseWindow(licenseTitle, licenseContents, promptText)
	if err := licenseWindow.Setup(); err != nil {
		return false, err
	}

	defer licenseWindow.Gui.Close()

	return licenseWindow.PromptUser()
}

func NewLicenseWindow(licenseTitle, licenseContents, promptText string) *LicenseWindowType {
	licenseWindow := &LicenseWindowType{}
	licenseHeight := utils.CountLines(licenseContents)
	licenseMarginBottom := 10

	// The LayoutManager is called on events, like key press or window resize
	// and it is going to look more or less like
	//
	// +-- license file name -------+
	// |  License contents line 1   |
	// |  License contents line 2   |
	// |  License contents line N   |
	// +----------------------------+
	// +----------------------------+
	// | promptText                 |
	// +----------------------------+
	licenseWindow.LayoutManager = func(g *gocui.Gui) error {
		terminalWidth, terminalHeight := g.Size()

		marginSize := 1
		promptWindowHeight := 3

		// License window dimensions
		licenseWindowBeginX := marginSize
		licenseWindowBeginY := marginSize
		licenseWindowEndX := terminalWidth - marginSize
		licenseWindowEndY := terminalHeight - marginSize - promptWindowHeight
		if v, err := g.SetView("license", licenseWindowBeginX, licenseWindowBeginY, licenseWindowEndX, licenseWindowEndY); err != nil {
			if err != gocui.ErrUnknownView {
				log.Error("Cannot modify license window: ", err)
				return err
			}
			v.Wrap = true
			v.Title = licenseTitle
			fmt.Fprint(v, strings.Replace(licenseContents, "\r", "", -1))
		}

		// Prompt window dimensions
		promptWindowBeginX := licenseWindowBeginX
		promptWindowBeginY := licenseWindowEndY + marginSize
		promptWindowEndX := licenseWindowEndX
		promptWindowEndY := terminalHeight - marginSize
		if v, err := g.SetView("prompt", promptWindowBeginX, promptWindowBeginY, promptWindowEndX, promptWindowEndY); err != nil {
			if err != gocui.ErrUnknownView {
				log.Error("Cannot modify prompt window: ", err)
				return err
			}
			fmt.Fprint(v, promptText)
		}

		_, err := g.SetCurrentView("license")
		if err != nil && err != gocui.ErrUnknownView {
			return err
		}

		if LicenseAgreed != nil {
			return gocui.ErrQuit
		}

		if Extract {
			return errs.ErrExtractEula
		}

		return nil
	}

	licenseWindow.ScrollUp = func(g *gocui.Gui, v *gocui.View) error {
		return licenseWindow.Scroll(v, -1)
	}

	licenseWindow.ScrollDown = func(g *gocui.Gui, v *gocui.View) error {
		return licenseWindow.Scroll(v, 1)
	}

	licenseWindow.Scroll = func(v *gocui.View, dy int) error {
		if v != nil {
			ox, oy := v.Origin()
			_, terminalHeight := licenseWindow.Gui.Size()
			y := oy + dy
			if y < 0 || y+terminalHeight-licenseMarginBottom >= licenseHeight {
				return nil
			}
			if err := v.SetOrigin(ox, oy+dy); err != nil {
				log.Errorf("Cannot scroll to %v, %v: %v", ox, oy+dy, err.Error())
				return err
			}
		}
		return nil
	}

	licenseWindow.Agree = func(g *gocui.Gui, v *gocui.View) error {
		LicenseAgreed = &Agreed
		return gocui.ErrQuit
	}

	licenseWindow.Disagree = func(g *gocui.Gui, v *gocui.View) error {
		LicenseAgreed = &Disagreed
		return gocui.ErrQuit
	}

	licenseWindow.Extract = func(g *gocui.Gui, v *gocui.View) error {
		Extract = true
		return gocui.ErrQuit
	}

	return licenseWindow
}

func (l *LicenseWindowType) Setup() error {
	log.Debug("Setting up UI to display license")
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		log.Error("Cannot initialize UI: ", err)
		return err
	}

	g.SetManagerFunc(l.LayoutManager)

	bindings := []struct {
		key         interface{}
		funcPointer func(g *gocui.Gui, v *gocui.View) error
	}{
		// Agree with 'a' or 'A'
		{'a', l.Agree},
		{'A', l.Agree},

		// Disagree with 'd'  or 'D'
		{'d', l.Disagree},
		{'D', l.Disagree},

		// Extract with 'e'  or 'E'
		{'e', l.Extract},
		{'E', l.Extract},

		// Scroll up with mouse/page/arrow up
		{gocui.MouseWheelUp, l.ScrollUp},
		{gocui.KeyArrowUp, l.ScrollUp},
		{gocui.KeyPgup, l.ScrollUp},

		// Scroll down with mouse/page/arrow down, enter or space
		{gocui.MouseWheelDown, l.ScrollDown},
		{gocui.KeyPgdn, l.ScrollDown},
		{gocui.KeyArrowDown, l.ScrollDown},
		{gocui.KeyEnter, l.ScrollDown},
		{gocui.KeySpace, l.ScrollDown},

		// Exit with Ctrl+C
		{gocui.KeyCtrlC, quit},
	}

	for _, binding := range bindings {
		_ = g.SetKeybinding("license", binding.key, gocui.ModNone, binding.funcPointer)
	}

	l.Gui = g
	return nil
}

func (l *LicenseWindowType) PromptUser() (bool, error) {
	log.Debug("Prompting user for license agreement")
	err := l.Gui.MainLoop()
	if err != nil && err != gocui.ErrQuit && err != errs.ErrExtractEula {
		log.Error("Cannot obtain user response: ", err)
		return false, err
	}

	if LicenseAgreed != nil {
		return *LicenseAgreed, nil
	}

	if Extract {
		return false, errs.ErrExtractEula
	}

	return false, nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	log.Warn("Aborting license agreement")
	return gocui.ErrQuit
}
