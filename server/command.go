package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/pkg/errors"
)

const (
	commandHelp = `* |/mstmeetings start| - Start an MS Teams meeting.
	* |/mstmeetings disconnect| - Disconnect from Mattermost`
	tooManyParametersText = "Too many parameters."
)

func getCommand() *model.Command {
	return &model.Command{
		Trigger:          "mstmeetings",
		DisplayName:      "MS Teams Meetings",
		Description:      "Integration with MS Teams Meetings.",
		AutoComplete:     true,
		AutoCompleteDesc: "Available commands: start, disconnect",
		AutoCompleteHint: "[command]",
	}
}

func (p *Plugin) postCommandResponse(args *model.CommandArgs, text string) {
	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: args.ChannelId,
		Message:   text,
	}
	_ = p.API.SendEphemeralPost(args.UserId, post)
}

func (p *Plugin) executeCommand(c *plugin.Context, args *model.CommandArgs) (string, error) {
	split := strings.Fields(args.Command)
	command := split[0]
	action := ""

	if command != "/mstmeetings" {
		return fmt.Sprintf("Command '%s' is not /mstmeetings. Please try again.", command), nil
	}

	if len(split) > 1 {
		action = split[1]
	} else {
		return p.handleHelp(split, args)
	}

	switch action {
	case "start":
		return p.handleStartConfirm(split[1:], args)
	case "disconnect":
		return p.handleDisconnect(split[1:], args)
	case "help":
		return p.handleHelp(split[1:], args)
	}

	return fmt.Sprintf("Unknown action `%v`.\n%s", action, p.getHelpText()), nil
}

func (p *Plugin) getHelpText() string {
	return "###### Mattermost MS Teams Meetings Plugin - Slash Command Help\n" + strings.ReplaceAll(commandHelp, "|", "`")
}

func (p *Plugin) handleHelp(args []string, extra *model.CommandArgs) (string, error) {
	return p.getHelpText(), nil
}

func (p *Plugin) handleStartConfirm(args []string, extra *model.CommandArgs) (string, error) {
	if len(args) > 1 {
		return tooManyParametersText, nil
	}
	userID := extra.UserId
	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		return "User not found.", errors.Wrap(appErr, "cannot get user")
	}
	channelID := extra.ChannelId
	channel, err := p.API.GetChannel(channelID)
	if err != nil {
		return "", err
	}
	message := ""
	if channel.IsGroupOrDirect() {
		var members *model.ChannelMembers
		members, err = p.API.GetChannelMembers(channelID, 0, 100)
		if err != nil {
			return "", err
		}

		if members != nil {
			p.API.LogDebug(fmt.Sprintf("%d members in channel %s", len(*members), channelID))
			membersCount := len(*members)
			message += "\n" + fmt.Sprintf("You are about a create a meeting in a channel with %d members", membersCount)
		}
	}

	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: channelID,
		Message:   message,
		Type:      "custom_mstmeetings",
		Props: map[string]interface{}{
			"type":                     "custom_mstmeetings",
			"meeting_status":           postTypeDialogWarn,
			"meeting_personal":         true,
			"meeting_creator_username": user.Username,
			"meeting_provider":         msteamsProviderName,
			"message":                  message,
		},
	}
	_, appErr = p.API.CreatePost(post)
	if appErr != nil {
		return "", appErr
	}

	return "", nil
}

func (p *Plugin) handleDisconnect(args []string, extra *model.CommandArgs) (string, error) {
	if len(args) > 1 {
		return tooManyParametersText, nil
	}
	err := p.disconnect(extra.UserId)
	if err != nil {
		return "Failed to disconnect the user, err=" + err.Error(), nil
	}

	p.trackDisconnect(extra.UserId)
	return "User disconnected from MS Teams Meetings.", nil
}

// ExecuteCommand is called when any registered by this plugin command is executed
func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	msg, err := p.executeCommand(c, args)
	if err != nil {
		p.API.LogWarn("failed to execute command", "error", err.Error())
	}
	if msg != "" {
		p.postCommandResponse(args, msg)
	}
	return &model.CommandResponse{}, nil
}
