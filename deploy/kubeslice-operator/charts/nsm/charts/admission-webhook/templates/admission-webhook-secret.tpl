apiVersion: v1
kind: Secret
metadata:
  name: nsm-admission-webhook-certs
  namespace: {{ .Release.Namespace }}
type: Opaque
data:
  tls.key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBd2Q3bXBuMjVNcFcyaVduUVpXbW5TbVZaM2lJZ3dGczdTQXZNK2hrdlBTRW5mZ2NFCnpuR2ZNeHFVaE93ZHRkVFhoR1Y0Sm9hYkloY3YrU003UEZid09PRldISG8rY3h5Zm1RWE1iRGF4SWRudU5Fc0kKdCtEQTFmdnJaZkZTQ2JEZG5McC9tcUNzOUJjMkNja0dZUDNqY1hyVFA2ekQ1RGUzRVlWdCt6L3FyaVZ0Nk8wdAppTXNuWXdRYnlTNHk4Z0JxZHRwWDB1RjJZdW93RGFlY3VnTFA3ZVhlaExTaURXazRDb3VXRDRHS0lESm40empZCi9jS09sZ3FnYkpDcko2MU1obks5WVlPdHBMQ01MWUVOaGZZV3BreUc3KzNFSVA5d3JuZ05aaUsrQUk4b1FCTkkKMFBKbWg1V0lZQVdZU1pXUi9xZGNmVllUZW5oRndLTklLbkVjOHdJREFRQUJBb0lCQUFOajNIQ28zaVl1VEFUWApIdGZISXkrLzJmUnljRlFzeERxY1NqZE5YWEFhTmxDVDJ0ZXBVUGxaeTZNUFplMmFEVEs1ZTRKZzlER0Nha3BXCi9XQXV2UUNob0JuYllXQXQ3ZlNGRDNBTS9NZjB3WitVZUZDTzA1QnFXVkZ0Q053MmhZbUtFVlVvM2gxZWtvbFYKUkpGSm4wS0t2VXJ0d0hjcktqNWFNUFFseC9ySGtPdFEzWC9KUGtZdnM2QUYwUWVkVm93djZkZ09wSXkweVlsbAphLzE3TU95eVYzRWtOeWJsK2lJQ2tFbnBYQmo4K1NFTjFSYitYZldjUWx1WGxwa2Z3bUIybmhLL3BCQmo4aGRiCjNEYStxd0tScmdudEZTcDQ4VEx0MXN2ZkcxYWRKQ2U5RkpGT2xhK0lXNXhiaUg0Z2RuaXc5YkxjQnZKeURTUVoKdnJEZ05lRUNnWUVBdzl6WUg3cmJzbytIbXN1VU1ac0ZZVkpKWEFHSlpvSFNBS1ovdFRnRnVyQXgrMUMxRFhEUApob2J6bEJobmRmTGVwUW95akZ6TlZ2ZEcyRDE0Q0hJZTk5ekUrUGdIZEs3OE5WUWVieEhDbmRGa05YUG8vSjU4CkhSMGVqanpaNTdKdDgrUzl3S3F6cHl5bmZic2lESiszNjlJNGpKM3VzWkxtaVNZTzNEVVNwMHNDZ1lFQS9XVjgKRTNlSzYxa1RqZ0l4ckNJdDduRk8yc05pcGlQUHdRSnJEcDgyTGRhcGxLL1VuL3FkN2tjZndtcms2VFdUbjR2MwpFN1JLVm5QVGJSTm9rSzNoaXF3akpmTW5tZWIxZGRUdHFRcVE1dWhwVEtoeC9vL2tXV2VSWjR3SzZaUXFhamcxCnZoOUpQNDFud1RhNlFLNlJIaGptNkFub2l1Ym1STlhOYm9BZEQva0NnWUFzTGlHMkxwRW1Hd2dzbTZWRzl6L2sKYndwTExiR1BwTkw1QUpXb0RBWUcwWDNFd2JURlhtQUJhV21DUzJyekNTQzl1Nm9oVFVHb1QwajB1QkRlWHRlcgpjMm9lK3R1N3Iwa0d3bjNHOGd1alM3czk3M0pyb1ZnL3ZQVEtndUZvU1RCU0pwUEM1UDUzUkRSWHdTRnlGWWtJCk1iZzl4OVl5eWY4a2lxZ3BkZk5LTndLQmdRRDZqUHNuUUg5eS8rdk84YXB0Mm9ueUI4V0JsOG9XSHJqUXpuUk8KeSt4RlhNam5CUWpIZW9Yb1VoazhJbmZmaENOSWtadW50dy85OVo3cmJsSnBKQlVzQ2RMak5rOUU1TkoyUlNrTgppUzRIczJ4UzZRZDJQbzc2TytiUkxPNnBVT0N6a0lyTFI2SWtuY3dtaHRlWkYwTFVNS2s0Ykh1cnhHMlJTSnBOCkZZNG0rUUtCZ0VMQ1ZHVVhuUU1PQitEY2hOcGwrZWNqZSszRmtRRHZiYWJra2dKaFlUajR2L3ZYcjV2MVlGbEgKL2ZqMWw2WC9WOEJSVkZhb1JMRE9FbGxHSzZxOHcyU1N1aGF4SXg3SUV5cXRKS1hzN0o5endUY1ZCMjZqZCtuUApScmVrT3JCKzFoSnBkM2ZlNVVvL2UvNmY5TS9RNWZsU1RSckFSMVJKYlNiRmx6VGpBanBDCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
  tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURjakNDQWxxZ0F3SUJBZ0lRTG4wWSt0WWpZMGM5bS9od3FKdUhYekFOQmdrcWhraUc5dzBCQVFzRkFEQWkKTVNBd0hnWURWUVFERXhkaFpHMXBjM05wYjI0dFkyOXVkSEp2Ykd4bGNpMWpZVEFlRncweU1UQTJNamd4TURVMApNekJhRncwek1UQTJNall4TURVME16QmFNQ1F4SWpBZ0JnTlZCQU1UR1c1emJTMWhaRzFwYzNOcGIyNHRkMlZpCmFHOXZheTF6ZG1Nd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUURCM3VhbWZia3kKbGJhSmFkQmxhYWRLWlZuZUlpREFXenRJQzh6NkdTODlJU2QrQndUT2NaOHpHcFNFN0IyMTFOZUVaWGdtaHBzaQpGeS81SXpzOFZ2QTQ0VlljZWo1ekhKK1pCY3hzTnJFaDJlNDBTd2kzNE1EVisrdGw4VklKc04yY3VuK2FvS3owCkZ6WUp5UVpnL2VOeGV0TS9yTVBrTjdjUmhXMzdQK3F1SlczbzdTMkl5eWRqQkJ2SkxqTHlBR3AyMmxmUzRYWmkKNmpBTnA1eTZBcy90NWQ2RXRLSU5hVGdLaTVZUGdZb2dNbWZqT05qOXdvNldDcUJza0tzbnJVeUdjcjFoZzYyawpzSXd0Z1EyRjloYW1USWJ2N2NRZy8zQ3VlQTFtSXI0QWp5aEFFMGpROG1hSGxZaGdCWmhKbFpIK3AxeDlWaE42CmVFWEFvMGdxY1J6ekFnTUJBQUdqZ2FFd2daNHdEZ1lEVlIwUEFRSC9CQVFEQWdXZ01CMEdBMVVkSlFRV01CUUcKQ0NzR0FRVUZCd01CQmdnckJnRUZCUWNEQWpBTUJnTlZIUk1CQWY4RUFqQUFNRjhHQTFVZEVRUllNRmFDSjI1egpiUzFoWkcxcGMzTnBiMjR0ZDJWaWFHOXZheTF6ZG1NdVlYWmxjMmhoTFhONWMzUmxiWUlyYm5OdExXRmtiV2x6CmMybHZiaTEzWldKb2IyOXJMWE4yWXk1aGRtVnphR0V0YzNsemRHVnRMbk4yWXpBTkJna3Foa2lHOXcwQkFRc0YKQUFPQ0FRRUFMekRqUTZ4UmlLQTBoQlpjVDZpWkt3V0RFU2Q2b3NybGVhaHJSWVpmc2hIcTl5MHNFUHUwS2c4bwpVWjRmZng0T0lhUkhQaUZPNTdrMnM4UFdqSkFZbXVDa0dCdW5sKy9oS2NyS1lwRXBYT25HNHFCVWx2NTgvN21qCmxaeGhsU0xDUERsTS9PVVgxdGROekk4cmxFY3RyalBMUEpFWGxjcW9lQnl5eG1xeWZhVFRDbTJqOXc5bm14QngKMkIxYXJNQVhJc3B2MEZWY0JpYkMxZUc1ZjJidm50cFVUL3ZtSk9wZXJNcVlMaWJieE1hWjJUSkthVUIyOWJNVQpYQVdHa0dURmRtbkprSENTQmxHK3NkK3k2VTZ4NUZnTWcyTWVYVTIzNGNEb2QvRDNrUDdYMEtwZEUwcUVSM2hKCkwvREh0Z3ZIa3JFNzNyR253M2wrNjRBL1BkeUJFZz09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nsm-admission-webhook
  namespace: {{ .Release.Namespace }}
  labels:
    app: nsm-admission-webhook
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nsm-admission-webhook
  template:
    metadata:
      labels:
        app: nsm-admission-webhook
    spec:
      containers:
        - name: nsm-admission-webhook
          image: {{ .Values.registry }}/{{ .Values.org }}/admission-webhook:{{ .Values.tag }}
          imagePullPolicy: {{ .Values.pullPolicy }}
          env:
            - name: REPO
              value: "{{ .Values.org }}"
            - name: TAG
              value: "{{ .Values.tag }}"
            - name: NSM_NAMESPACE
              value: "{{ .Values.clientNamespace }}"
            - name: TRACER_ENABLED
              value: {{ .Values.global.JaegerTracing | default false | quote }}
            - name: JAEGER_AGENT_HOST
              value: jaeger.{{ .Release.Namespace }}
            - name: JAEGER_AGENT_PORT
              value: "6831"
          volumeMounts:
            - name: webhook-certs
              mountPath: /etc/webhook/certs
              readOnly: true
          livenessProbe:
            httpGet:
              path: /liveness
              port: 5555
            initialDelaySeconds: 10
            periodSeconds: 10
            timeoutSeconds: 3
          readinessProbe:
            httpGet:
              path: /readiness
              port: 5555
            initialDelaySeconds: 10
            periodSeconds: 10
            timeoutSeconds: 3
      volumes:
        - name: webhook-certs
          secret:
            secretName: nsm-admission-webhook-certs
---
apiVersion: v1
kind: Service
metadata:
  name: nsm-admission-webhook-svc
  namespace: {{ .Release.Namespace }}
  labels:
    app: nsm-admission-webhook
spec:
  ports:
    - port: 443
      targetPort: 443
  selector:
    app: nsm-admission-webhook
---
apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingWebhookConfiguration
metadata:
  name: nsm-admission-webhook-cfg
  namespace: {{ .Release.Namespace }}
  labels:
    app: nsm-admission-webhook
webhooks:
  - name: admission-webhook.networkservicemesh.io
    clientConfig:
      service:
        name: nsm-admission-webhook-svc
        namespace: {{ .Release.Namespace }}
        path: "/mutate"
      caBundle: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURFVENDQWZtZ0F3SUJBZ0lSQUwyQUxJYXRnZGxLeXZDRlNNQ2pyQVV3RFFZSktvWklodmNOQVFFTEJRQXcKSWpFZ01CNEdBMVVFQXhNWFlXUnRhWE56YVc5dUxXTnZiblJ5YjJ4c1pYSXRZMkV3SGhjTk1qRXdOakk0TVRBMQpORE13V2hjTk16RXdOakkyTVRBMU5ETXdXakFpTVNBd0hnWURWUVFERXhkaFpHMXBjM05wYjI0dFkyOXVkSEp2CmJHeGxjaTFqWVRDQ0FTSXdEUVlKS29aSWh2Y05BUUVCQlFBRGdnRVBBRENDQVFvQ2dnRUJBT1NsMitFZ282SVkKOXIweXZUWURCZitDa0hZL2JTTkprOU9jMVFNWGpxb1NIUUVRYTZFZERyak1aNENtWmpKVFRzZGJmTjJBK1hqRQp4VnRQdFRvMHlFUjNYQk9aTUQzR1FGcyszekFTU2tqU1dCRENDMFFOYnMybDU1elhEWE9CNnRtcUFDRlF4ZlMwClZYYnlucHdUS1kzSENQUnp2Yzl2d0ZaZ05VdU9MekUwNE11cjdpVC9VNXFYQmQ1Y012SWZXNHIwTTZCQTdJMmsKaU1PNWNtdmlWZnBjR3BJRU5WVGhKd3RUSksvZW8zeHRDMmlnRUY2L1dLL05wam1FZnFUaks0OWRHYWd4UXdIeQplS0dvTFpuN2FYeXh2QlhTYXRkSWZDTEVOZU9vTHdBTlRDNG5NcUtLUHdlbGhwUHF6KzF0S1pqeG04MjdqNFBhCjRzbFVXRWJ3UytNQ0F3RUFBYU5DTUVBd0RnWURWUjBQQVFIL0JBUURBZ0trTUIwR0ExVWRKUVFXTUJRR0NDc0cKQVFVRkJ3TUJCZ2dyQmdFRkJRY0RBakFQQmdOVkhSTUJBZjhFQlRBREFRSC9NQTBHQ1NxR1NJYjNEUUVCQ3dVQQpBNElCQVFDbUIvSGRvWFdLclYzVnhaUlo5dmN0Z0J2azBqV1MyeVdVNUxlWERPbUtjOGxzdFdGaDRMcnhEV0lNCkkxR2R3NTBNSTBLaW81cW4zZ3NXUHU0MTRFVkFZUm1Oc2FyekxJOEpvdklTNFI2ZEZqMDdlN0drK3F5ZnJKODYKV1NjQWxEdmRzY2JDVXVEVGxRUWVRT3JwQ1djVWpHbWtqanZncm15d2lHajZrQzJEajJhdVQ3bkZnOXBvQWJFUwpnVTc1ak9nZlM3ZHQ2WDdlaGRlMHFQR2JCL3VCVmxUemRZRTcycEFTWVhUaEl1UkJNUmlWbXJ6bUxvNzFRMTh2CkhDK0cyVktzQk5OSHVBTnNDUHF0YXEzWkFWK2hRUStKSUxJK2VmZVh3eGp2WERXdVlwY2I2SXJZSU83clpSRW0KSW4rL2drdlAyWmNlSWVXN1VvZDMvU1cxVlZtRgotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
    rules:
      - operations: ["CREATE"]
        apiGroups: ["apps", "extensions", ""]
        apiVersions: ["v1", "v1beta1"]
        resources: ["deployments", "services", "pods"]
