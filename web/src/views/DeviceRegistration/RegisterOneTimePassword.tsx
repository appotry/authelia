import React, { useEffect, useCallback, useState } from "react";

import { IconDefinition, faCopy, faKey, faTimesCircle } from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";
import { makeStyles, Typography, Button, IconButton, Link, CircularProgress, TextField } from "@material-ui/core";
import { red } from "@material-ui/core/colors";
import classnames from "classnames";
import QRCode from "qrcode.react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate } from "react-router-dom";

import AppStoreBadges from "@components/AppStoreBadges";
import { GoogleAuthenticator } from "@constants/constants";
import { IndexRoute } from "@constants/Routes";
import { useNotifications } from "@hooks/NotificationsContext";
import LoginLayout from "@layouts/LoginLayout";
import { completeTOTPRegistrationProcess } from "@services/RegisterDevice";
import { extractIdentityToken } from "@utils/IdentityToken";

const RegisterOneTimePassword = function () {
    const style = useStyles();
    const navigate = useNavigate();
    const location = useLocation();
    // The secret retrieved from the API is all is ok.
    const [secretURL, setSecretURL] = useState("empty");
    const [secretBase32, setSecretBase32] = useState(undefined as string | undefined);
    const { createSuccessNotification, createErrorNotification } = useNotifications();
    const [hasErrored, setHasErrored] = useState(false);
    const [isLoading, setIsLoading] = useState(false);
    const { t: translate } = useTranslation("Portal");

    // Get the token from the query param to give it back to the API when requesting
    // the secret for OTP.
    const processToken = extractIdentityToken(location.search);

    const handleDoneClick = () => {
        navigate(IndexRoute);
    };

    const completeRegistrationProcess = useCallback(async () => {
        if (!processToken) {
            return;
        }

        setIsLoading(true);
        try {
            const secret = await completeTOTPRegistrationProcess(processToken);
            setSecretURL(secret.otpauth_url);
            setSecretBase32(secret.base32_secret);
        } catch (err) {
            console.error(err);
            if ((err as Error).message.includes("Request failed with status code 403")) {
                createErrorNotification(
                    translate(
                        "You must open the link from the same device and browser that initiated the registration process",
                    ),
                );
            } else {
                createErrorNotification(
                    translate("Failed to register device, the provided link is expired or has already been used"),
                );
            }
            setHasErrored(true);
        }
        setIsLoading(false);
    }, [processToken, createErrorNotification, translate]);

    useEffect(() => {
        completeRegistrationProcess();
    }, [completeRegistrationProcess]);

    function SecretButton(text: string | undefined, action: string, icon: IconDefinition) {
        return (
            <IconButton
                className={style.secretButtons}
                color="primary"
                onClick={() => {
                    navigator.clipboard.writeText(`${text}`);
                    createSuccessNotification(`${action}`);
                }}
            >
                <FontAwesomeIcon icon={icon} />
            </IconButton>
        );
    }
    const qrcodeFuzzyStyle = isLoading || hasErrored ? style.fuzzy : undefined;

    return (
        <LoginLayout title={translate("Scan QR Code")}>
            <div className={style.root}>
                <div className={style.googleAuthenticator}>
                    <Typography className={style.googleAuthenticatorText}>
                        {translate("Need Google Authenticator?")}
                    </Typography>
                    <AppStoreBadges
                        iconSize={128}
                        targetBlank
                        className={style.googleAuthenticatorBadges}
                        googlePlayLink={GoogleAuthenticator.googlePlay}
                        appleStoreLink={GoogleAuthenticator.appleStore}
                    />
                </div>
                <div className={classnames(qrcodeFuzzyStyle, style.qrcodeContainer)}>
                    <Link href={secretURL}>
                        <QRCode value={secretURL} className={style.qrcode} size={256} />
                        {!hasErrored && isLoading ? <CircularProgress className={style.loader} size={128} /> : null}
                        {hasErrored ? <FontAwesomeIcon className={style.failureIcon} icon={faTimesCircle} /> : null}
                    </Link>
                </div>
                <div>
                    {secretURL !== "empty" ? (
                        <TextField
                            id="secret-url"
                            label={translate("Secret")}
                            className={style.secret}
                            value={secretURL}
                            InputProps={{
                                readOnly: true,
                            }}
                        />
                    ) : null}
                    {secretBase32
                        ? SecretButton(secretBase32, translate("OTP Secret copied to clipboard"), faKey)
                        : null}
                    {secretURL !== "empty"
                        ? SecretButton(secretURL, translate("OTP URL copied to clipboard"), faCopy)
                        : null}
                </div>
                <Button
                    variant="contained"
                    color="primary"
                    className={style.doneButton}
                    onClick={handleDoneClick}
                    disabled={isLoading}
                >
                    {translate("Done")}
                </Button>
            </div>
        </LoginLayout>
    );
};

export default RegisterOneTimePassword;

const useStyles = makeStyles((theme) => ({
    root: {
        paddingTop: theme.spacing(4),
        paddingBottom: theme.spacing(4),
    },
    qrcode: {
        marginTop: theme.spacing(2),
        marginBottom: theme.spacing(2),
        padding: theme.spacing(),
        backgroundColor: "white",
    },
    fuzzy: {
        filter: "blur(10px)",
    },
    secret: {
        marginTop: theme.spacing(1),
        marginBottom: theme.spacing(1),
        width: "256px",
    },
    googleAuthenticator: {},
    googleAuthenticatorText: {
        fontSize: theme.typography.fontSize * 0.8,
    },
    googleAuthenticatorBadges: {},
    secretButtons: {
        width: "128px",
    },
    doneButton: {
        width: "256px",
    },
    qrcodeContainer: {
        position: "relative",
        display: "inline-block",
    },
    loader: {
        position: "absolute",
        top: "calc(128px - 64px)",
        left: "calc(128px - 64px)",
        color: "rgba(255, 255, 255, 0.5)",
    },
    failureIcon: {
        position: "absolute",
        top: "calc(128px - 64px)",
        left: "calc(128px - 64px)",
        color: red[400],
        fontSize: "128px",
    },
}));
