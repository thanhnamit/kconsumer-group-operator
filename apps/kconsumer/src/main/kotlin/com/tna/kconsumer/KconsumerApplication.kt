package com.tna.kconsumer

import org.apache.kafka.clients.consumer.ConsumerConfig
import org.apache.kafka.common.serialization.StringDeserializer
import org.slf4j.Logger
import org.slf4j.LoggerFactory
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.beans.factory.annotation.Value
import org.springframework.boot.CommandLineRunner
import org.springframework.boot.autoconfigure.SpringBootApplication
import org.springframework.boot.runApplication
import org.springframework.context.annotation.Bean
import org.springframework.kafka.annotation.KafkaListener
import org.springframework.kafka.config.KafkaListenerContainerFactory
import org.springframework.kafka.core.ConsumerFactory
import org.springframework.kafka.core.DefaultKafkaConsumerFactory
import org.springframework.kafka.core.KafkaTemplate
import org.springframework.kafka.listener.ContainerProperties
import org.springframework.kafka.listener.GenericMessageListenerContainer
import org.springframework.kafka.listener.KafkaMessageListenerContainer
import java.util.concurrent.TimeUnit

@SpringBootApplication
class KconsumerApplication {
	private val logger: Logger = LoggerFactory.getLogger(KconsumerApplication::class.java)

	@KafkaListener(topics = ["\${topic}"])
	fun consumerMessage(msg: String) {
		if (msg == "baz")
			logger.info("COMPLETED: {}", msg)
		Thread.sleep(10)
	}
}

fun main(args: Array<String>) {
	runApplication<KconsumerApplication>(*args)
}
