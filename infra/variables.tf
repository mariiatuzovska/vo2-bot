variable "postgres_user" {
  type    = string
  default = "vo2"
}

variable "postgres_db" {
  type    = string
  default = "vo2"
}

variable "postgres_password" {
  type      = string
  sensitive = true
}

variable "postgres_port" {
  type    = number
  default = 5432
}
