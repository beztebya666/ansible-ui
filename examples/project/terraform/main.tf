terraform {
  required_version = ">= 1.4.0"
}

# A localhost-safe demo: terraform_data is a built-in managed resource (no
# provider downloads, no cloud credentials). The extra-var `greeting` arrives
# as TF_VAR_greeting from the run's variables.
variable "greeting" {
  type        = string
  default     = "hello from terraform"
  description = "Message echoed by the apply step."
}

resource "terraform_data" "demo" {
  input = var.greeting

  provisioner "local-exec" {
    command = "echo '[terraform] applying: ${self.input}'"
  }
}

output "message" {
  value = terraform_data.demo.output
}

output "ran_at" {
  value = timestamp()
}
