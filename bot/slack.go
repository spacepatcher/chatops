package bot

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/devopsext/chatops/common"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/devopsext/utils"
	"github.com/slack-go/slack"
	"github.com/slack-io/slacker"
)

type SlackOptions struct {
	BotToken        string
	AppToken        string
	Debug           bool
	ReactionDoing   string
	ReactionDone    string
	ReactionFailed  string
	ReactionDialog  string
	DefaultCommand  string
	HelpCommand     string
	Permisssions    string
	Timeout         int
	PublicChannel   string
	AttachmentColor string
	ErrorColor      string
}

type SlackUser struct {
	id   string
	name string
}

type SlackFileResponseFull struct {
	slack.File   `json:"file"`
	slack.Paging `json:"paging"`
	Comments     []slack.Comment        `json:"comments"`
	Files        []slack.File           `json:"files"`
	Metadata     slack.ResponseMetadata `json:"response_metadata"`
	slack.SlackResponse
}

type SlackUploadURLExternalResponse struct {
	UploadURL string `json:"upload_url"`
	FileID    string `json:"file_id"`
	slack.SlackResponse
}

type SlackCompleteUploadExternalResponse struct {
	Files []slack.FileSummary `json:"files"`
	slack.SlackResponse
}

type Slack struct {
	options           SlackOptions
	processors        *common.Processors
	client            *slacker.Slacker
	auth              *slack.AuthTestResponse
	logger            sreCommon.Logger
	meter             sreCommon.Meter
	defaultDefinition *slacker.CommandDefinition
	helpDefinition    *slacker.CommandDefinition
}

type SlackRichTextQuoteElement struct {
	Type   slack.RichTextElementType `json:"type"`
	Text   string                    `json:"text,omitempty"`
	UserID string                    `json:"user_id,omitempty"`
}

type SlackRichTextQuote struct {
	Type     slack.RichTextElementType    `json:"type"`
	Elements []*SlackRichTextQuoteElement `json:"elements"`
}

type SlackFile struct {
	URL string `json:"url,omitempty"`
	ID  string `json:"id,omitempty"`
}

type SlackImageBlock struct {
	Type      slack.MessageBlockType `json:"type"`
	SlackFile *SlackFile             `json:"slack_file"`
	AltText   string                 `json:"alt_text"`
	BlockID   string                 `json:"block_id,omitempty"`
	Title     *slack.TextBlockObject `json:"title,omitempty"`
}

type SlackButtonValue struct {
	Timestamp string
	Text      string
}

type slackMessageInfo struct {
	typ             string
	text            string
	userID          string
	channelID       string
	timestamp       string
	threadTimestamp string
}

const (
	slackAPIURL                      = "https://slack.com/api/"
	slackFilesGetUploadURLExternal   = "files.getUploadURLExternal"
	slackFilesCompleteUploadExternal = "files.completeUploadExternal"
	slackFilesSharedPublicURL        = "files.sharedPublicURL"
	slackMaxTextBlockLength          = 3000
	slackSubmitAction                = "submit"
	slackCancelAction                = "cancel"
)

// SlackRichTextQuote
func (r SlackRichTextQuote) RichTextElementType() slack.RichTextElementType {
	return r.Type
}

func (r SlackRichTextQuoteElement) RichTextElementType() slack.RichTextElementType {
	return r.Type
}

// SlackImageBlock
func (s SlackImageBlock) BlockType() slack.MessageBlockType {
	return s.Type
}

// SlackUser

func (su *SlackUser) ID() string {
	return su.id
}

func (su *SlackUser) Name() string {
	return su.name
}

// Slack

func (s *Slack) Name() string {
	return "Slack"
}

/*func (s *Slack) Info() interface{} {

	if s.auth == nil {
		return nil
	}
	return s.auth
}*/

func (s *Slack) getEventTextCommand(command string, m *slackMessageInfo) (string, string) {

	text := m.text
	if m.typ == "slash_commands" {
		text = strings.TrimSpace(text)
	} else {
		items := strings.SplitAfter(text, ">")

		if len(items) > 1 {
			text = strings.TrimSpace(items[1])
		}
	}

	arr := strings.Split(text, " ")
	if len(arr) > 0 {
		command = strings.TrimSpace(arr[0])
	}

	return text, command
}

/*
func (s *Slack) getBotAuth() string {

	auth := ""
	if !utils.IsEmpty(s.options.BotToken) {
		auth = fmt.Sprintf("Bearer %s", s.options.BotToken)
		return auth
	}
	return auth
}

func (s *Slack) getUserAuth() string {

	auth := ""
	if !utils.IsEmpty(s.options.UserToken) {
		auth = fmt.Sprintf("Bearer %s", s.options.UserToken)
		return auth
	}
	return auth
}


func (s *Slack) ShareFilePublicURL(file *slack.File) (*slack.File, error) {

	client := utils.NewHttpSecureClient(15)

	params := url.Values{}
	params.Add("file", file.ID)

	u, err := url.Parse(slackAPIURL)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, slackFilesSharedPublicURL)
	u.RawQuery = params.Encode()

	data, err := utils.HttpPostRaw(client, u.String(), "application/x-www-form-urlencoded", s.getUserAuth(), nil)
	if err != nil {
		return nil, err
	}

	r := &SlackFileResponseFull{}
	err = json.Unmarshal(data, r)
	if err != nil {
		return nil, err
	}
	if !r.SlackResponse.Ok {
		return nil, errors.New(r.SlackResponse.Error)
	}

	return &r.File, nil
}
*/

func (s *Slack) uploadFileV1(att *common.Attachment) (string, error) {

	botID := "unknown"
	if s.auth != nil {
		botID = s.auth.BotID
	}
	stamp := time.Now().Format("20060102T150405")
	name := fmt.Sprintf("%s-%s", botID, stamp)
	fileParams := slack.FileUploadParameters{
		Filename: name,
		Reader:   bytes.NewReader(att.Data),
		Channels: []string{s.options.PublicChannel},
	}
	private, err := s.client.SlackClient().UploadFile(fileParams)
	if err != nil {
		return "", err
	}
	return private.ID, nil
}

func (s *Slack) limitText(text string, max int) string {
	r := text
	l := len(text)
	trimmed := "...trimmed :broken_heart:"
	l2 := len(trimmed)
	if l > max {
		r = fmt.Sprintf("%s%s", r[0:max-l2-1], trimmed)
	}
	return r
}

func (s *Slack) buildAttachmentBlocks(attachments []*common.Attachment) ([]slack.Attachment, error) {

	r := []slack.Attachment{}
	for _, a := range attachments {

		blks := []slack.Block{}

		switch a.Type {
		case common.AttachmentTypeImage:

			// uploading image
			id, err := s.uploadFileV1(a)
			if err != nil {
				return r, err
			}

			blks = append(blks, &SlackImageBlock{
				Type:    slack.MBTImage,
				AltText: a.Text,
				Title: &slack.TextBlockObject{
					Type: slack.PlainTextType, // only
					Text: a.Title,
				},
				SlackFile: &SlackFile{ID: id},
			})

		default:

			// title
			if !utils.IsEmpty(a.Title) {
				blks = append(blks,
					slack.NewSectionBlock(
						slack.NewTextBlockObject(slack.MarkdownType, string(a.Title), false, false),
						[]*slack.TextBlockObject{}, nil,
					))
			}

			// body
			if !utils.IsEmpty(a.Data) {

				blks = append(blks,
					slack.NewSectionBlock(
						slack.NewTextBlockObject(slack.MarkdownType, s.limitText(string(a.Data), slackMaxTextBlockLength), false, false),
						[]*slack.TextBlockObject{}, nil,
					))
			}
		}
		r = append(r, slack.Attachment{
			Color: s.options.AttachmentColor,
			Blocks: slack.Blocks{
				BlockSet: blks,
			},
		})
	}
	return r, nil
}

func (s *Slack) addReaction(m *slackMessageInfo, name string) {

	if m.typ == "slash_commands" {
		return
	}
	err := s.client.SlackClient().AddReaction(name, slack.NewRefToMessage(m.channelID, m.timestamp))
	if err != nil {
		s.logger.Error("Slack adding reaction error: %s", err)
	}
}

func (s *Slack) removeReaction(m *slackMessageInfo, name string) {

	if m.typ == "slash_commands" {
		return
	}

	s.client.SlackClient()

	err := s.client.SlackClient().RemoveReaction(name, slack.NewRefToMessage(m.channelID, m.timestamp))
	if err != nil {
		s.logger.Error("Slack removing reaction error: %s", err)
	}
}

func (s *Slack) addRemoveReactions(m *slackMessageInfo, first, second string) {
	s.addReaction(m, first)
	s.removeReaction(m, second)
}

func (s *Slack) findGroup(groups []slack.UserGroup, userID string, group *regexp.Regexp) *slack.UserGroup {

	for _, g := range groups {

		match := group.MatchString(g.Name)
		if match && utils.Contains(g.Users, userID) {
			return &g
		}
	}
	return nil
}

// .*=^(help|news|app|application|catalog)$,some=^(escalate)$
func (s *Slack) denyAccess(userID string, command string) bool {

	if utils.IsEmpty(s.options.Permisssions) {
		return false
	}

	groups, err := s.client.SlackClient().GetUserGroups(slack.GetUserGroupsOptionIncludeCount(true), slack.GetUserGroupsOptionIncludeUsers(true))
	if err != nil {
		s.logger.Error("Slack getting user group error: %s", err)
		return false
	}

	permissions := utils.MapGetKeyValues(s.options.Permisssions)
	for group, value := range permissions {

		reCommand, err := regexp.Compile(value)
		if err != nil {
			s.logger.Error("Slack command regex error: %s", err)
			return true
		}

		mCommand := reCommand.MatchString(command)
		if !mCommand {
			continue
		}

		reGroup, err := regexp.Compile(group)
		if err != nil {
			s.logger.Error("Slack group regex error: %s", err)
			return true
		}

		mGroup := s.findGroup(groups, userID, reGroup)
		if mGroup != nil {
			return false
		}
	}
	return true
}

func (s *Slack) matchParam(text, param string) map[string]string {

	r := make(map[string]string)
	re := regexp.MustCompile(param)
	match := re.FindStringSubmatch(text)
	if len(match) == 0 {
		return r
	}

	names := re.SubexpNames()
	for i, name := range names {
		if i != 0 && name != "" {
			r[name] = match[i]
		}
	}
	return r
}

func (s *Slack) findParams(command string, params []string, m *slackMessageInfo) common.ExecuteParams {

	r := make(common.ExecuteParams)

	if utils.IsEmpty(params) {
		return r
	}

	text, command := s.getEventTextCommand(command, m)
	arr := strings.SplitAfter(text, command)
	if len(arr) < 2 {
		return r
	}
	text = strings.TrimSpace(arr[1])

	for _, p := range params {
		values := s.matchParam(text, p)
		for k, v := range values {
			r[k] = v
		}
		if len(r) > 0 {
			return r
		}
	}

	return r
}

func (s *Slack) updateCounters(group, command, text, userID string) {

	labels := make(map[string]string)
	if !utils.IsEmpty(group) {
		labels["group"] = group
	}
	if !utils.IsEmpty(text) {
		labels["command"] = command
	}
	if !utils.IsEmpty(text) {
		labels["text"] = text
	}
	labels["user_id"] = userID

	s.meter.Counter("requests", "Count of all requests", labels, "slack", "bot").Inc()
}

func (s *Slack) unsupportedCommandHandler(cc *slacker.CommandContext) {

	text := cc.Event().Text
	items := strings.Split(text, ">")
	if len(items) > 1 {
		text = strings.TrimSpace(items[1])
	}

	if utils.IsEmpty(text) && s.helpDefinition != nil {
		s.helpDefinition.Handler(cc)
		return
	}

	if s.defaultDefinition != nil {
		s.defaultDefinition.Handler(cc)
		return
	}
	s.updateCounters("", "", text, cc.Event().UserID)
}

func (s *Slack) reply(command string, m *slackMessageInfo,
	replier *slacker.ResponseReplier, message string, attachments []*common.Attachment,
	response common.Response, start *time.Time, error bool) error {

	threadTS := m.threadTimestamp
	text, _ := s.getEventTextCommand(command, m)
	replyInThread := !utils.IsEmpty(threadTS)

	atts := []slack.Attachment{}
	opts := []slacker.PostOption{}
	if error {
		atts = append(atts, slack.Attachment{
			Color: s.options.ErrorColor,
			Blocks: slack.Blocks{
				BlockSet: []slack.Block{
					slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, message, false, false),
						[]*slack.TextBlockObject{}, nil),
				},
			},
		})
		opts = append(opts, slacker.SetAttachments(atts))
	} else {
		batts, err := s.buildAttachmentBlocks(attachments)
		if err != nil {
			return err
		}
		opts = append(opts, slacker.SetAttachments(batts))
	}

	if replyInThread {
		opts = append(opts, slacker.SetThreadTS(threadTS))
	}

	if !response.Visible {
		opts = append(opts, slacker.SetEphemeral(m.userID))
	}

	var quote = []*SlackRichTextQuoteElement{}

	var durationElement *SlackRichTextQuoteElement
	if start != nil && !error && response.Duration {

		elapsed := time.Since(*start)
		durationElement = &SlackRichTextQuoteElement{
			Type: "text",
			Text: fmt.Sprintf("[%s] ", elapsed.Round(time.Millisecond)),
		}
		quote = append(quote, durationElement)
	}

	blocks := []slack.Block{}

	if response.Original {

		quote = append(quote, []*SlackRichTextQuoteElement{
			{Type: "user", UserID: m.userID},
			{Type: "text", Text: fmt.Sprintf(" %s", text)},
		}...)

		elements := []slack.RichTextElement{
			// add quote
			&SlackRichTextQuote{Type: slack.RTEQuote, Elements: quote},
		}
		blocks = append(blocks, slack.NewRichTextBlock("quote", elements...))
	}

	if !error {
		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, message, false, false),
			[]*slack.TextBlockObject{}, nil,
		))
	}

	_, err := replier.PostBlocks(m.channelID, blocks, opts...)
	if err != nil {
		return err
	}
	return nil
}

func (s *Slack) replyError(command string, m *slackMessageInfo,
	replier *slacker.ResponseReplier, err error, attachments []*common.Attachment) error {

	s.logger.Error("Slack reply error: %s", err)
	return s.reply(command, m, replier, err.Error(), attachments, common.Response{Visible: false}, nil, true)
}

func (s *Slack) getInteractionID(command, group string) string {

	if utils.IsEmpty(group) {
		return command
	}
	return fmt.Sprintf("%s-%s", command, group)
}

func (s *Slack) replyInteraction(command, group string, fields []common.Field, params common.ExecuteParams,
	m *slackMessageInfo, replier *slacker.ResponseReplier) (bool, error) {

	threadTS := m.threadTimestamp
	opts := []slacker.PostOption{}
	replyInThread := !utils.IsEmpty(threadTS)
	if replyInThread {
		opts = append(opts, slacker.SetThreadTS(threadTS))
	}

	opts = append(opts, slacker.SetEphemeral(m.userID))
	blocks := []slack.Block{}
	interactionID := s.getInteractionID(command, group)

	for _, field := range fields {

		actionID := fmt.Sprintf("%s-%s", interactionID, field.Name)
		def := params[field.Name]
		if utils.IsEmpty(def) {
			def = field.Default
		}

		l := slack.NewTextBlockObject(slack.PlainTextType, field.Label, false, false)
		var h *slack.TextBlockObject
		if !utils.IsEmpty(field.Hint) {
			h = slack.NewTextBlockObject(slack.PlainTextType, field.Hint, false, false)
		}

		var b *slack.InputBlock
		var el slack.BlockElement

		switch field.Type {
		case common.FieldTypeMultiEdit:
			e := slack.NewPlainTextInputBlockElement(h, actionID)
			e.Multiline = true
			e.InitialValue = def
			el = e
		case common.FieldTypeURL:
			e := slack.NewURLTextInputBlockElement(h, actionID)
			e.InitialValue = def
			el = e
		case common.FieldTypeDate:
			e := slack.NewDatePickerBlockElement(actionID)
			e.InitialDate = time.Now().Format("2006-01-02")
			el = e
		case common.FieldTypeSelect:
			options := []*slack.OptionBlockObject{}
			var dBlock *slack.OptionBlockObject
			for _, v := range field.Values {
				block := slack.NewOptionBlockObject(v, slack.NewTextBlockObject(slack.PlainTextType, v, false, false), h)
				if v == def {
					dBlock = block
				}
				options = append(options, block)
			}
			e := slack.NewOptionsSelectBlockElement(slack.OptTypeStatic, h, actionID, options...)
			if dBlock != nil {
				e.InitialOption = dBlock
			}
			el = e
		case common.FieldTypeMultiSelect:
			options := []*slack.OptionBlockObject{}
			dBlocks := []*slack.OptionBlockObject{}
			arr := common.RemoveEmptyStrings(strings.Split(def, ","))
			for _, v := range field.Values {
				block := slack.NewOptionBlockObject(v, slack.NewTextBlockObject(slack.PlainTextType, v, false, false), h)
				if utils.Contains(arr, v) {
					dBlocks = append(dBlocks, block)
				}
				options = append(options, block)
			}
			e := slack.NewOptionsMultiSelectBlockElement(slack.MultiOptTypeStatic, h, actionID, options...)
			if len(dBlocks) > 0 {
				e.InitialOptions = dBlocks
			}
			el = e
		default:
			e := slack.NewPlainTextInputBlockElement(h, actionID)
			e.InitialValue = def
			el = e
		}

		b = slack.NewInputBlock("", l, nil, el)
		if b != nil {
			blocks = append(blocks, b)
		}
	}

	if len(blocks) == 0 {
		return false, nil
	}

	// pass message timestamp & text to each button
	value := &SlackButtonValue{
		Timestamp: m.timestamp,
		Text:      m.text,
	}
	data, err := json.Marshal(value)
	if err != nil {
		return false, err
	}
	sv := base64.StdEncoding.EncodeToString(data)

	submit := slack.NewButtonBlockElement(slackSubmitAction, sv, slack.NewTextBlockObject(slack.PlainTextType, "Submit", false, false))
	cancel := slack.NewButtonBlockElement(slackCancelAction, sv, slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false))

	ab := slack.NewActionBlock(interactionID, submit, cancel)
	blocks = append(blocks, ab)

	s.addReaction(m, s.options.ReactionDialog)
	_, err = replier.PostBlocks(m.channelID, blocks, opts...)
	if err != nil {
		s.removeReaction(m, s.options.ReactionDialog)
		return false, err
	}
	return true, nil
}

func (s *Slack) postCommand(cmd common.Command, m *slackMessageInfo, userProfile *slack.UserProfile,
	replier *slacker.ResponseReplier, params common.ExecuteParams, error bool) bool {

	cName := cmd.Name()
	response := cmd.Response()

	user := &SlackUser{
		id: m.userID,
	}
	if userProfile != nil {
		user.name = userProfile.DisplayName
	}

	s.addReaction(m, s.options.ReactionDoing)

	start := time.Now()
	message, attachments, err := cmd.Execute(s, user, params)
	if err != nil {
		s.replyError(cName, m, replier, err, attachments)
		s.addRemoveReactions(m, s.options.ReactionFailed, s.options.ReactionDoing)
		return false
	}

	err = s.reply(cName, m, replier, message, attachments, response, &start, false)
	if err != nil {
		s.replyError(cName, m, replier, err, attachments)
		s.addRemoveReactions(m, s.options.ReactionFailed, s.options.ReactionDoing)
		return false
	}

	if error {
		s.addRemoveReactions(m, s.options.ReactionFailed, s.options.ReactionDoing)
	} else {
		s.addRemoveReactions(m, s.options.ReactionDone, s.options.ReactionDoing)
	}

	return true
}

func (s *Slack) defCommandDefinition(cmd common.Command, group string, error bool) *slacker.CommandDefinition {

	cName := cmd.Name()
	params := cmd.Params()
	fields := cmd.Fields()
	def := &slacker.CommandDefinition{
		Command:     cName,
		Aliases:     cmd.Aliases(),
		Description: cmd.Description(),
		HideHelp:    true,
	}
	def.Handler = func(cc *slacker.CommandContext) {

		event := cc.Event()
		m := &slackMessageInfo{
			typ:             event.Type,
			text:            event.Text,
			userID:          event.UserID,
			channelID:       event.ChannelID,
			timestamp:       event.TimeStamp,
			threadTimestamp: event.ThreadTimeStamp,
		}

		replier := cc.Response()

		text, _ := s.getEventTextCommand(cName, m)
		s.updateCounters(group, cName, text, m.userID)

		groupName := cName
		if !utils.IsEmpty(group) {
			groupName = fmt.Sprintf("%s/%s", group, cName)
		}

		if (def != s.defaultDefinition) && (def != s.helpDefinition) {
			if s.denyAccess(m.userID, groupName) {
				s.logger.Debug("Slack user %s is not permitted to execute %s", m.userID, groupName)
				s.unsupportedCommandHandler(cc)
				return
			}
		}

		eParams := s.findParams(cName, params, m)
		if len(fields) > len(eParams) {
			shown, err := s.replyInteraction(cName, group, fields, eParams, m, replier)
			if err != nil {
				s.replyError(cName, m, replier, err, []*common.Attachment{})
				s.addRemoveReactions(m, s.options.ReactionFailed, s.options.ReactionDoing)
				return
			}
			if shown {
				return
			}
		}

		s.postCommand(cmd, m, event.UserProfile, replier, eParams, error)
	}
	return def
}

func (s *Slack) hideInteraction(m *slackMessageInfo, responseURL string) {
	s.client.SlackClient().PostEphemeral(m.channelID, m.userID,
		slack.MsgOptionReplaceOriginal(responseURL),
		slack.MsgOptionDeleteOriginal(responseURL),
	)
}

func (s *Slack) defInteractionDefinition(cmd common.Command, group string) *slacker.InteractionDefinition {

	cName := cmd.Name()
	interactionID := s.getInteractionID(cName, group)
	def := &slacker.InteractionDefinition{
		InteractionID: interactionID,
		Type:          slack.InteractionTypeBlockActions,
	}
	def.Handler = func(ic *slacker.InteractionContext) {

		callback := ic.Callback()
		replier := ic.Response()

		m := &slackMessageInfo{
			typ:             callback.Container.Type,
			text:            "", // get this from button value
			userID:          callback.User.ID,
			channelID:       callback.Container.ChannelID,
			timestamp:       "",                          // get this from button value
			threadTimestamp: callback.Container.ThreadTs, // keep thread TS
		}

		actions := callback.ActionCallback.BlockActions
		if len(actions) == 0 {
			s.logger.Error("Slack actions are not defined.")
			s.removeReaction(m, s.options.ReactionDialog)
			return
		}

		action := actions[0]

		data, err := base64.StdEncoding.DecodeString(action.Value)
		if err != nil {
			s.removeReaction(m, s.options.ReactionDialog)
			return
		}

		value := &SlackButtonValue{}
		err = json.Unmarshal(data, value)
		if err != nil {
			s.removeReaction(m, s.options.ReactionDialog)
			return
		}

		m.timestamp = value.Timestamp // this is original message TS
		m.text = value.Text           // this is original message text

		s.hideInteraction(m, callback.ResponseURL)
		s.removeReaction(m, s.options.ReactionDialog)

		switch action.ActionID {
		case slackSubmitAction:
			userProfileParams := &slack.GetUserProfileParameters{
				UserID: m.userID,
			}
			userProfile, err := s.client.SlackClient().GetUserProfile(userProfileParams)
			if err != nil {
				s.logger.Error("Slack couldn't get user profile for %s: %s", m.userID, err)
			}

			params := make(common.ExecuteParams)
			states := callback.BlockActionState
			if states != nil && len(states.Values) > 0 {

				for _, v1 := range states.Values {
					for k2, v2 := range v1 {
						name := strings.Replace(k2, fmt.Sprintf("%s-", interactionID), "", 1)

						v := v2.Value
						switch v2.Type {
						case "datepicker":
							v = v2.SelectedDate
						case "static_select":
							v = v2.SelectedOption.Value
						case "multi_static_select":
							arr := []string{}
							for _, v2 := range v2.SelectedOptions {
								arr = append(arr, v2.Value)
							}
							v = strings.Join(arr, ",")
						}
						params[name] = v
					}
				}
			}

			s.postCommand(cmd, m, userProfile, replier, params, false)
		default:
			s.addReaction(m, s.options.ReactionFailed)
		}
	}
	return def
}

func (s *Slack) Debug(msg string, args ...any) {
	s.logger.Debug(msg, args...)
}

func (s *Slack) Info(msg string, args ...any) {
	s.logger.Info(msg, args...)
}

func (s *Slack) Warn(msg string, args ...any) {
	s.logger.Warn(msg, args...)
}

func (s *Slack) Error(msg string, args ...any) {
	s.logger.Error(msg, args...)
}

func (s *Slack) start() {

	options := []slacker.ClientOption{
		slacker.WithDebug(s.options.Debug),
		slacker.WithLogger(s),
	}
	client := slacker.NewClient(s.options.BotToken, s.options.AppToken, options...)
	client.UnsupportedCommandHandler(s.unsupportedCommandHandler)

	s.defaultDefinition = nil
	s.helpDefinition = nil

	items := s.processors.Items()
	// add groups firstly
	for _, p := range items {

		pName := p.Name()
		commands := p.Commands()
		var group *slacker.CommandGroup

		if utils.IsEmpty(pName) {
			continue
		}
		group = client.AddCommandGroup(pName)
		for _, c := range commands {
			group.AddCommand(s.defCommandDefinition(c, pName, false))
			if len(c.Fields()) > 0 {
				client.AddInteraction(s.defInteractionDefinition(c, pName))
			}
		}
	}

	group := client.AddCommandGroup("")
	// add root secondly
	for _, p := range items {

		pName := p.Name()
		commands := p.Commands()

		if !utils.IsEmpty(pName) {
			continue
		}
		for _, c := range commands {
			name := c.Name()
			if name == s.options.DefaultCommand {
				s.defaultDefinition = s.defCommandDefinition(c, "", true)
			} else {
				def := s.defCommandDefinition(c, "", false)
				if name == s.options.HelpCommand {
					s.helpDefinition = def
					client.Help(def)
				}
				group.AddCommand(def)
				if len(c.Fields()) > 0 {
					client.AddInteraction(s.defInteractionDefinition(c, ""))
				}
			}
		}
	}

	s.client = client
	auth, err := client.SlackClient().AuthTest()
	if err == nil {
		s.auth = auth
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = client.Listen(ctx)
	if err != nil {
		s.logger.Error("Slack listen error: %s", err)
		return
	}
}

func (t *Slack) Start(wg *sync.WaitGroup) {

	if wg == nil {
		t.start()
		return
	}

	wg.Add(1)

	go func(wg *sync.WaitGroup) {

		defer wg.Done()
		t.start()
	}(wg)
}

func NewSlack(options SlackOptions, observability *common.Observability, processors *common.Processors) *Slack {

	return &Slack{
		options:    options,
		processors: processors,
		logger:     observability.Logs(),
		meter:      observability.Metrics(),
	}
}
