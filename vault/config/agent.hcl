# Vault Agent для сервиса main.
# Аутентифицируется через AppRole, выпускает клиентский TLS-сертификат из PKI
# и записывает его в /run/certs/. main использует его для исходящих gRPC-соединений.
# Сертификат обновляется автоматически за ~30% до истечения TTL.

auto_auth {
  method "approle" {
    config = {
      # Файлы записываются entrypoint-скриптом из env VAULT_ROLE_ID / VAULT_SECRET_ID
      role_id_file_path   = "/vault/auth/role-id"
      secret_id_file_path = "/vault/auth/secret-id"
    }
  }
}

# pkiCert выпускает один сертификат и использует его во всех трёх блоках template.
# Повторные вызовы с теми же аргументами возвращают закешированный результат.

template {
  contents    = <<EOT
{{ with pkiCert "pki_int/issue/main" "common_name=main.internal" "ttl=72h" }}{{ .Cert }}{{ end }}
EOT
  destination = "/run/certs/cert.pem"
  perms       = "0640"
}

template {
  contents    = <<EOT
{{ with pkiCert "pki_int/issue/main" "common_name=main.internal" "ttl=72h" }}{{ .Key }}{{ end }}
EOT
  destination = "/run/certs/key.pem"
  perms       = "0640"
}

template {
  contents    = <<EOT
{{ with pkiCert "pki_int/issue/main" "common_name=main.internal" "ttl=72h" }}{{ .CA }}{{ end }}
EOT
  destination = "/run/certs/ca.pem"
  perms       = "0644"
}
