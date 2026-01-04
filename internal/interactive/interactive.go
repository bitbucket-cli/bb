package interactive

import (
	"fmt"
	"os"
	"strings"

	"github.com/manifoldco/promptui"
)

// Option represents a selectable option
type Option struct {
	Label       string
	Value       string
	Description string
}

// Select prompts the user to select from a list of options
func Select(label string, options []Option) (Option, error) {
	if !IsInteractive() {
		return Option{}, fmt.Errorf("interactive mode not available (not a terminal)")
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "\U0001F449 {{ .Label | cyan }}",
		Inactive: "  {{ .Label }}",
		Selected: "\U00002705 {{ .Label | green }}",
		Details: `
--------- Details ----------
{{ "Label:" | faint }}	{{ .Label }}
{{ if .Description }}{{ "Description:" | faint }}	{{ .Description }}{{ end }}`,
	}

	searcher := func(input string, index int) bool {
		option := options[index]
		name := strings.ToLower(option.Label)
		input = strings.ToLower(input)
		return strings.Contains(name, input)
	}

	prompt := promptui.Select{
		Label:     label,
		Items:     options,
		Templates: templates,
		Size:      10,
		Searcher:  searcher,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return Option{}, err
	}

	return options[idx], nil
}

// SelectString prompts the user to select from a list of strings
func SelectString(label string, items []string) (string, error) {
	if !IsInteractive() {
		return "", fmt.Errorf("interactive mode not available (not a terminal)")
	}

	prompt := promptui.Select{
		Label: label,
		Items: items,
		Size:  10,
	}

	_, result, err := prompt.Run()
	return result, err
}

// Confirm prompts the user for a yes/no confirmation
func Confirm(label string) (bool, error) {
	if !IsInteractive() {
		return false, fmt.Errorf("interactive mode not available (not a terminal)")
	}

	prompt := promptui.Prompt{
		Label:     label,
		IsConfirm: true,
	}

	_, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// Input prompts the user for text input
func Input(label string, defaultValue string) (string, error) {
	if !IsInteractive() {
		return "", fmt.Errorf("interactive mode not available (not a terminal)")
	}

	prompt := promptui.Prompt{
		Label:   label,
		Default: defaultValue,
	}

	return prompt.Run()
}

// InputSecret prompts the user for secret input (hidden)
func InputSecret(label string) (string, error) {
	if !IsInteractive() {
		return "", fmt.Errorf("interactive mode not available (not a terminal)")
	}

	prompt := promptui.Prompt{
		Label: label,
		Mask:  '*',
	}

	return prompt.Run()
}

// IsInteractive returns true if stdin is a terminal
func IsInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// MultiSelect prompts the user to select multiple options
func MultiSelect(label string, options []Option) ([]Option, error) {
	if !IsInteractive() {
		return nil, fmt.Errorf("interactive mode not available (not a terminal)")
	}

	var selected []Option
	remaining := make([]Option, len(options))
	copy(remaining, options)

	for {
		// Add "Done" option
		selectOptions := make([]Option, len(remaining)+1)
		selectOptions[0] = Option{Label: "[Done - finish selection]", Value: "__done__"}
		copy(selectOptions[1:], remaining)

		templates := &promptui.SelectTemplates{
			Label:    "{{ . }}",
			Active:   "\U0001F449 {{ .Label | cyan }}",
			Inactive: "  {{ .Label }}",
			Selected: "\U00002705 {{ .Label | green }}",
		}

		prompt := promptui.Select{
			Label:     fmt.Sprintf("%s (selected: %d)", label, len(selected)),
			Items:     selectOptions,
			Templates: templates,
			Size:      10,
		}

		idx, _, err := prompt.Run()
		if err != nil {
			return selected, err
		}

		if idx == 0 {
			// Done selected
			break
		}

		// Add to selected, remove from remaining
		selectedOpt := selectOptions[idx]
		selected = append(selected, selectedOpt)

		// Remove from remaining
		newRemaining := make([]Option, 0, len(remaining)-1)
		for _, opt := range remaining {
			if opt.Value != selectedOpt.Value {
				newRemaining = append(newRemaining, opt)
			}
		}
		remaining = newRemaining

		if len(remaining) == 0 {
			break
		}
	}

	return selected, nil
}
