package com.tna.kproducer

import org.slf4j.Logger
import org.slf4j.LoggerFactory
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.beans.factory.annotation.Value
import org.springframework.boot.autoconfigure.SpringBootApplication
import org.springframework.boot.runApplication
import org.springframework.context.annotation.Bean
import org.springframework.kafka.core.KafkaTemplate
import org.springframework.web.servlet.function.ServerResponse
import org.springframework.web.servlet.function.router
import java.util.*

@SpringBootApplication
class KproducerApplication {
	private val logger: Logger = LoggerFactory.getLogger(KproducerApplication::class.java)

	@Value("\${topic}")
	lateinit var topicName: String

	@Bean
	fun createApi(@Autowired template: KafkaTemplate<String, String>) = router {
		GET("/send/{record}") {
			val recordTotal = it.pathVariable("record").toInt()
			for (i in 1..recordTotal) {
				template.send(topicName, "bar-$i")
			}
			template.send(topicName, "baz")
			logger.info("Sent all messages")
			ServerResponse.ok().body("Sent $recordTotal + 1")
		}
	}
}

fun main(args: Array<String>) {
	runApplication<KproducerApplication>(*args)
}
