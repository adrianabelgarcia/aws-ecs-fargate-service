# Runbook: Deployment rollback

**Alarm or symptom:** a release is serving errors or latency, or a CodeDeploy
deployment is stuck in progress. Typical triggers are `5xx-fast-burn`,
`latency-p99`, or `unhealthy-hosts` firing during or after a deployment.

**Severity:** P1 if user-facing errors are above the fast-burn threshold, P2 otherwise.

## 1. Decide which path applies

| State | Path |
|-------|------|
| A deployment is `InProgress` (canary or traffic shifting) and an alarm is in ALARM | Path A: stop it with automatic rollback. Usually this is already happening. |
| The deployment has `Succeeded` less than `termination_wait_minutes` ago | Path B: roll back to the blue tasks |
| The deployment has `Succeeded` and the old tasks are gone | Path C: deploy the previous revision |

```bash
aws deploy list-deployments \
  --application-name "$APP" --deployment-group-name "$GROUP" \
  --include-only-statuses InProgress Succeeded Failed Stopped --max-items 5

aws deploy get-deployment --deployment-id d-XXXXXXXXX \
  --query 'deploymentInfo.{status:status,error:errorInformation,rollback:rollbackInformation}'
```

## 2. Path A: stop an in-progress deployment and roll back

```bash
aws deploy stop-deployment --deployment-id d-XXXXXXXXX --auto-rollback-enabled
```

CodeDeploy moves the listeners back to the blue target group and stops the
green tasks. Confirm with `get-deployment` until `status` is `Stopped` and
`rollbackInformation.status` is `SUCCEEDED`.

## 3. Path B: roll back during the termination window

After a successful deployment, the old (blue) tasks keep running for
`termination_wait_minutes` (5 minutes in both environments). CodeDeploy does not
expose a one-call traffic revert once a deployment has succeeded, so the
fastest supported route is Path C: a new deployment of the previous revision.
Start it immediately. The old tasks are still warm, so the shift is quick.

## 4. Path C: redeploy the previous revision

1. Find the live revision and the one before it.

   ```bash
   LIVE=$(aws ecs describe-services --cluster "$CLUSTER" --services "$SERVICE" \
     --query 'services[0].taskDefinition' --output text)
   echo "live: $LIVE"
   FAMILY=${LIVE%:*}; REV=${LIVE##*:}
   PREV="$FAMILY:$((REV - 1))"
   aws ecs describe-task-definition --task-definition "$PREV" \
     --query 'taskDefinition.containerDefinitions[0].image' --output text
   ```

2. Write an AppSpec that points at the previous revision, and wrap it as a CodeDeploy revision.

   ```bash
   cat > /tmp/appspec-rollback.yaml <<YAML
   version: 0.0
   Resources:
     - TargetService:
         Type: AWS::ECS::Service
         Properties:
           TaskDefinition: arn:aws:ecs:${AWS_REGION}:ACCOUNT_ID:task-definition/${PREV}
           LoadBalancerInfo:
             ContainerName: "${SERVICE}"
             ContainerPort: 8080
           PlatformVersion: "LATEST"
   YAML
   jq -n --arg c "$(cat /tmp/appspec-rollback.yaml)" \
     '{revisionType: "AppSpecContent", appSpecContent: {content: $c}}' > /tmp/revision.json
   ```

   Replace `ACCOUNT_ID` with the account ID (the ARN is the only place it is needed).

3. Start the deployment.

   ```bash
   aws deploy create-deployment --application-name "$APP" --deployment-group-name "$GROUP" \
     --revision file:///tmp/revision.json --description "rollback to $PREV"
   ```

4. Remove the temporary files: `rm -f /tmp/appspec-rollback.yaml /tmp/revision.json`.

Two rules apply during any rollback:

- Do not run `aws ecs update-service` on this service. The service uses the `CODE_DEPLOY` deployment controller, and CodeDeploy owns the task definition.
- Do not run `terraform apply` to roll back. Terraform ignores the task definition, so it will not revert a release. Fix the image through a release.

## 5. Verify

```bash
make smoke BASE_URL="$BASE_URL"
aws deploy get-deployment --deployment-id d-XXXXXXXXX --query 'deploymentInfo.status'
```

Confirm `5xx-fast-burn` returns to OK and p99 is back under the threshold. Check
the dashboard `${ENV}` service view. Leave the failing commit reverted on `main` by a
follow-up PR so the next pipeline run does not ship it again.

## 6. Afterwards

- Open an incident note with the deployment ID, the commit SHA, and the alarm timeline.
- Keep the failing image in ECR. Do not delete it. The lifecycle policy keeps the last 50 tags.
- If the canary did not catch the regression, add the missing check to the smoke test or to the alarms.

## Related

- [ADR 0002: CodeDeploy blue/green](../adr/0002-codedeploy-blue-green-over-rolling.md)
- [5xx spike](5xx-spike.md)
