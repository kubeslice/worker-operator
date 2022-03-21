@Library('jenkins-library@feature/helm-pipeline') _
dockerImagePipeline1(
  script: this,
  service: 'kubeslice/operator',
  dockerfile: 'Dockerfile',
  buildContext: '.',
  buildArguments: [PLATFORM:"amd64"],
  trigger_remote: 'yes',
  remote_job: 'realtimeai/kubeslice-mesh/master'
) 