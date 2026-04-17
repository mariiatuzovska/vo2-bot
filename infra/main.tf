resource "docker_network" "vo2" {
  name = "vo2-net"
}

resource "docker_volume" "postgres_data" {
  name = "vo2-postgres-data"
}

resource "docker_image" "postgres" {
  name = "postgres:16"
}

resource "docker_container" "postgres" {
  image = docker_image.postgres.image_id
  name  = "vo2-postgres"

  env = [
    "POSTGRES_USER=${var.postgres_user}",
    "POSTGRES_PASSWORD=${var.postgres_password}",
    "POSTGRES_DB=${var.postgres_db}",
  ]

  ports {
    internal = 5432
    external = var.postgres_port
  }

  volumes {
    volume_name    = docker_volume.postgres_data.name
    container_path = "/var/lib/postgresql/data"
  }

  networks_advanced {
    name = docker_network.vo2.name
  }

  restart = "unless-stopped"

  healthcheck {
    test     = ["CMD-SHELL", "pg_isready -U ${var.postgres_user} -d ${var.postgres_db}"]
    interval = "10s"
    timeout  = "5s"
    retries  = 5
  }
}
