package aws_ses

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/pulsoats/core/domain/derrors"
	"github.com/pulsoats/core/lib/errorsx"
	"github.com/pulsoats/core/lib/logx"
	"github.com/pulsoats/main/internal/domain/mailer"
)

type Client struct {
	client *sesv2.Client
	sender string
}

type Config struct {
	BaseEndpoint string
	Region       string
	AccessKey    string
	SecretKey    string
	Sender       string

	Logger logx.Logger
}

func NewClient(cfg Config) (mailer.Sender, error) {
	if cfg.Region == "" {
		return nil, fmt.Errorf("aws sesv2 client: %w: region", derrors.ErrRequired)
	}
	if cfg.Sender == "" {
		return nil, fmt.Errorf("aws sesv2 client: %w: sender address", derrors.ErrRequired)
	}

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}
	if cfg.BaseEndpoint != "" {
		opts = append(opts, config.WithBaseEndpoint(cfg.BaseEndpoint))
	}
	if cfg.AccessKey != "" || cfg.SecretKey != "" {
		if cfg.AccessKey == "" || cfg.SecretKey == "" {
			return nil, fmt.Errorf("aws sesv2 client: %w: credentials", derrors.ErrRequired)
		}
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")))
	}
	if cfg.Logger != nil {
		opts = append(opts, config.WithLogger(awsLogger{log: cfg.Logger}))
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("aws sesv2 client: %w", err)
	}

	client := sesv2.NewFromConfig(awsCfg)

	return &Client{
		client: client,
		sender: cfg.Sender,
	}, nil
}

func (s *Client) Send(ctx context.Context, msg mailer.Message) error {
	if msg.To == "" {
		return fmt.Errorf("aws sesv2 client: %w: recipient", derrors.ErrRequired)
	}
	if msg.Subject == "" {
		return fmt.Errorf("aws sesv2 client: %w: subject", derrors.ErrRequired)
	}
	if msg.Text == "" && msg.HTML == "" {
		return fmt.Errorf("aws sesv2 client: %w: body", derrors.ErrRequired)
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
		return fmt.Errorf("aws sesv2 client: send email: %w: %v", errorsx.ErrInternal, err)
	}
	return nil
}
