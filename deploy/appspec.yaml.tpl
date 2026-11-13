# CodeDeploy AppSpec for ECS blue/green. Rendered by deploy/render-taskdef.sh.
#
# The aws-actions/amazon-ecs-deploy-task-definition action replaces
# <TASK_DEFINITION> with the ARN of the revision it registers. The placeholders
# __CONTAINER_NAME__ and __CONTAINER_PORT__ are filled by render-taskdef.sh.
#
# No lifecycle hooks are declared. Pre-traffic verification is done by the
# CodeDeploy test listener plus the post-deploy smoke test in the pipeline.
# Add AfterAllowTestTraffic / BeforeAllowTraffic Lambda hooks here if you need
# in-deployment verification (see docs/adr/0002-codedeploy-blue-green.md).
version: 0.0
Resources:
  - TargetService:
      Type: AWS::ECS::Service
      Properties:
        TaskDefinition: <TASK_DEFINITION>
        LoadBalancerInfo:
          ContainerName: "__CONTAINER_NAME__"
          ContainerPort: __CONTAINER_PORT__
        PlatformVersion: "LATEST"
