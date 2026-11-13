# Runbook: Secret rotation (API token)

**Covers:** the bearer token that protects write endpoints, stored at
`items-api-${ENV}/api-token` in Secrets Manager and injected into the container
as `API_TOKEN`.

**When:** on a schedule (every 90 days), after an exposure, or after an operator with access leaves.

**Severity:** P2 for scheduled rotation. P1 for suspected exposure.

## How the token reaches the app

- Terraform creates the secret container and the KMS encryption, but never the value. The value never enters Terraform state.
- ECS resolves `valueFrom` when a task starts. A running task keeps the value it started with.
- Therefore a rotation takes effect for new tasks only. Existing tasks keep the old token until a deployment replaces them.
- Read requests are public. Only `POST` requires the token.

## 1. Generate the new value without exposing it

Generate it on a trusted machine into a file that only you can read. Do not put
the value in a command line, a ticket, a chat, or CI logs.

```bash
umask 077
openssl rand -base64 32 | tr -d '\n' > "$HOME/.items-api-token.new"
```

## 2. Set the new value as the current secret version

```bash
aws secretsmanager put-secret-value \
  --secret-id "items-api-${ENV}/api-token" \
  --secret-string "file://$HOME/.items-api-token.new"

shred -u "$HOME/.items-api-token.new" 2>/dev/null || rm -P "$HOME/.items-api-token.new" 2>/dev/null || rm -f "$HOME/.items-api-token.new"
```

On macOS, `rm -P` overwrites and removes the file. Verify it no longer exists afterwards.

## 3. Redeploy so new tasks read the new value

Redeploy the same image. The task definition does not change, so this is a
revision-only CodeDeploy deployment, and the new tasks resolve the new
`AWSCURRENT` value at start.

```bash
LIVE=$(aws ecs describe-services --cluster "$CLUSTER" --services "$SERVICE" \
  --query 'services[0].taskDefinition' --output text)
# Build the AppSpec for $LIVE exactly as in the deployment rollback runbook (Path C, step 2),
# then start the deployment:
aws deploy create-deployment --application-name "$APP" --deployment-group-name "$GROUP" \
  --revision file:///tmp/revision.json --description "secret rotation: reload API token"
```

Wait for the deployment to succeed, and remove `/tmp/revision.json` afterwards.

## 4. Verify

```bash
# Old token must now be rejected
curl -s -o /dev/null -w '%{http_code}\n' -X POST -H "Authorization: Bearer ${OLD_TOKEN}" \
  -H 'Content-Type: application/json' -d '{"name":"rotation-check"}' "$BASE_URL/api/v1/items"   # expect 401

# New token must be accepted (use a throwaway item name and delete it afterwards if the API supports it)
curl -s -o /dev/null -w '%{http_code}\n' -X POST -H "Authorization: Bearer ${NEW_TOKEN}" \
  -H 'Content-Type: application/json' -d '{"name":"rotation-check"}' "$BASE_URL/api/v1/items"   # expect 201
```

Keep the test value in a shell variable only for the duration of the check, and
unset it afterwards. Run `make smoke BASE_URL="$BASE_URL"` as a final check.

## 5. Clean up and record

- Disable the previous version in Secrets Manager, or let it age out. Check which versions remain with `aws secretsmanager describe-secret`.
- Record the rotation date and the deployment ID in the security log.
- Update the consumers that hold the token (only trusted operators or automation should).

## Exposure response

If the token may have leaked:

1. Rotate immediately using the steps above.
2. Review application logs for `POST` requests with an unexpected source. The WAF logs redact the `Authorization` header, so token values are never logged.
3. Check WAF and ALB access logs in S3 for the period before the rotation.
4. Treat any writes made with the leaked token as unauthorised and assess them.

## Known limits

The service accepts one token. A rotation therefore needs one deployment, and
clients that use the old token fail between the secret change and their switch
to the new value. The planned improvement is two-token overlap, which accepts
`API_TOKEN` and `API_TOKEN_PREVIOUS` during the rotation window. See the
README's "What I would do next".
