package middlewares

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"

	"github.com/authelia/authelia/v4/internal/model"
	"github.com/authelia/authelia/v4/internal/templates"
)

// IdentityVerificationStart the handler for initiating the identity validation process.
func IdentityVerificationStart(args IdentityVerificationStartArgs, delayFunc TimingAttackDelayFunc) RequestHandler {
	if args.IdentityRetrieverFunc == nil {
		panic(fmt.Errorf("Identity verification requires an identity retriever"))
	}

	return func(ctx *AutheliaCtx) {
		requestTime := time.Now()
		success := false

		if delayFunc != nil {
			defer delayFunc(ctx.Logger, requestTime, &success)
		}

		identity, err := args.IdentityRetrieverFunc(ctx)
		if err != nil {
			// In that case we reply ok to avoid user enumeration.
			ctx.Logger.Error(err)
			ctx.ReplyOK()

			return
		}

		var jti uuid.UUID

		if jti, err = uuid.NewRandom(); err != nil {
			ctx.Error(err, messageOperationFailed)
			return
		}

		verification := model.NewIdentityVerification(jti, identity.Username, args.ActionClaim, ctx.RemoteIP())

		// Create the claim with the action to sign it.
		claims := verification.ToIdentityVerificationClaim()

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		ss, err := token.SignedString([]byte(ctx.Configuration.JWTSecret))

		if err != nil {
			ctx.Error(err, messageOperationFailed)
			return
		}

		err = ctx.Providers.StorageProvider.SaveIdentityVerification(ctx, verification)
		if err != nil {
			ctx.Error(err, messageOperationFailed)
			return
		}

		uri, err := ctx.ExternalRootURL()
		if err != nil {
			ctx.Error(err, messageOperationFailed)
			return
		}

		link := fmt.Sprintf("%s%s?token=%s", uri, args.TargetEndpoint, ss)

		bufHTML := new(bytes.Buffer)

		disableHTML := false
		if ctx.Configuration.Notifier != nil && ctx.Configuration.Notifier.SMTP != nil {
			disableHTML = ctx.Configuration.Notifier.SMTP.DisableHTMLEmails
		}

		if !disableHTML {
			htmlParams := map[string]interface{}{
				"title":  args.MailTitle,
				"url":    link,
				"button": args.MailButtonContent,
			}

			err = templates.HTMLEmailTemplate.Execute(bufHTML, htmlParams)

			if err != nil {
				ctx.Error(err, messageOperationFailed)
				return
			}
		}

		bufText := new(bytes.Buffer)
		textParams := map[string]interface{}{
			"url": link,
		}

		err = templates.PlainTextEmailTemplate.Execute(bufText, textParams)

		if err != nil {
			ctx.Error(err, messageOperationFailed)
			return
		}

		ctx.Logger.Debugf("Sending an email to user %s (%s) to confirm identity for registering a device.",
			identity.Username, identity.Email)

		err = ctx.Providers.Notifier.Send(identity.Email, args.MailTitle, bufText.String(), bufHTML.String())

		if err != nil {
			ctx.Error(err, messageOperationFailed)
			return
		}

		success = true

		ctx.ReplyOK()
	}
}

// IdentityVerificationFinish the middleware for finishing the identity validation process.
func IdentityVerificationFinish(args IdentityVerificationFinishArgs, next func(ctx *AutheliaCtx, username string)) RequestHandler {
	return func(ctx *AutheliaCtx) {
		var finishBody IdentityVerificationFinishBody

		b := ctx.PostBody()

		err := json.Unmarshal(b, &finishBody)

		if err != nil {
			ctx.Error(err, messageOperationFailed)
			return
		}

		if finishBody.Token == "" {
			ctx.Error(fmt.Errorf("No token provided"), messageOperationFailed)
			return
		}

		token, err := jwt.ParseWithClaims(finishBody.Token, &model.IdentityVerificationClaim{},
			func(token *jwt.Token) (interface{}, error) {
				return []byte(ctx.Configuration.JWTSecret), nil
			})

		if err != nil {
			if ve, ok := err.(*jwt.ValidationError); ok {
				switch {
				case ve.Errors&jwt.ValidationErrorMalformed != 0:
					ctx.Error(fmt.Errorf("Cannot parse token"), messageOperationFailed)
					return
				case ve.Errors&(jwt.ValidationErrorExpired|jwt.ValidationErrorNotValidYet) != 0:
					// Token is either expired or not active yet.
					ctx.Error(fmt.Errorf("Token expired"), messageIdentityVerificationTokenHasExpired)
					return
				default:
					ctx.Error(fmt.Errorf("Cannot handle this token: %s", ve), messageOperationFailed)
					return
				}
			}

			ctx.Error(err, messageOperationFailed)

			return
		}

		claims, ok := token.Claims.(*model.IdentityVerificationClaim)
		if !ok {
			ctx.Error(fmt.Errorf("Wrong type of claims (%T != *middlewares.IdentityVerificationClaim)", claims), messageOperationFailed)
			return
		}

		verification, err := claims.ToIdentityVerification()
		if err != nil {
			ctx.Error(fmt.Errorf("Token seems to be invalid: %w", err),
				messageOperationFailed)
			return
		}

		found, err := ctx.Providers.StorageProvider.FindIdentityVerification(ctx, verification.JTI.String())
		if err != nil {
			ctx.Error(err, messageOperationFailed)
			return
		}

		if !found {
			ctx.Error(fmt.Errorf("Token is not in DB, it might have already been used"),
				messageIdentityVerificationTokenAlreadyUsed)
			return
		}

		// Verify that the action claim in the token is the one expected for the given endpoint.
		if claims.Action != args.ActionClaim {
			ctx.Error(fmt.Errorf("This token has not been generated for this kind of action"), messageOperationFailed)
			return
		}

		if args.IsTokenUserValidFunc != nil && !args.IsTokenUserValidFunc(ctx, claims.Username) {
			ctx.Error(fmt.Errorf("This token has not been generated for this user"), messageOperationFailed)
			return
		}

		err = ctx.Providers.StorageProvider.ConsumeIdentityVerification(ctx, claims.ID, model.NewNullIP(ctx.RemoteIP()))
		if err != nil {
			ctx.Error(err, messageOperationFailed)
			return
		}

		next(ctx, claims.Username)
	}
}
