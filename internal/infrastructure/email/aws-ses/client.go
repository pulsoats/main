package aws_ses

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/domain/mailer"
)

type Client struct {
	client *sesv2.Client
	sender string
}

func NewClient(client *sesv2.Client, sender string) (mailer.Sender, error) {
	if client == nil {
		return nil, fmt.Errorf("aws sesv2 client is nil")
	}
	if sender == "" {
		return nil, fmt.Errorf("aws sesv2 client: sender address is empty")
	}
	return &Client{client: client, sender: sender}, nil
}

func (s *Client) Send(ctx context.Context, msg mailer.Message) error {
	const op = "aws sesv2 client"
	if msg.To == "" {
		return fmt.Errorf("%s: recipient: %w", op, errorsx.ErrInvalidArgument)
	}
	if msg.Subject == "" {
		return fmt.Errorf("%s: subject: %w", op, errorsx.ErrInvalidArgument)
	}
	if msg.Text == "" && msg.HTML == "" {
		return fmt.Errorf("%s: body: %w", op, errorsx.ErrInvalidArgument)
	}

	body := &types.Body{}
	if msg.HTML != "" {
		body.Html = &types.Content{
			Data: aws.String(msg.HTML),
		}
	}
	if msg.Text != "" {
		body.Text = &types.Content{
			Data: aws.String(msg.Text),
		}
	}

	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(s.sender),
		Destination: &types.Destination{
			ToAddresses: []string{msg.To},
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data: aws.String(msg.Subject),
				},
				Body: body,
			},
		},
	}

	if _, err := s.client.SendEmail(ctx, input); err != nil {
		return fmt.Errorf("%s: send email: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	return nil
}
