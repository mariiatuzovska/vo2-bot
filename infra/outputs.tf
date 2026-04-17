output "database_url" {
  value     = "postgres://${var.postgres_user}:${var.postgres_password}@localhost:${var.postgres_port}/${var.postgres_db}?sslmode=disable"
  sensitive = true
}
