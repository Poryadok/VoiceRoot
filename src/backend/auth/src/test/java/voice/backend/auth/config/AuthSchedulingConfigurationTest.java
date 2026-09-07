package voice.backend.auth.config;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;
import org.springframework.scheduling.annotation.ScheduledAnnotationBeanPostProcessor;

class AuthSchedulingConfigurationTest {
  private final ApplicationContextRunner contextRunner =
      new ApplicationContextRunner().withUserConfiguration(AuthSchedulingConfiguration.class);

  @Test
  void enablesSchedulingWhenThePropertyIsAbsent() {
    contextRunner.run(
        context ->
            assertThat(context)
                .hasSingleBean(ScheduledAnnotationBeanPostProcessor.class)
                .hasBean("org.springframework.context.annotation.internalScheduledAnnotationProcessor"));
  }

  @Test
  void doesNotEnableSchedulingWhenTheEnvironmentDisablesIt() {
    contextRunner
        .withPropertyValues("spring.task.scheduling.enabled=false")
        .run(
            context ->
                assertThat(context)
                    .doesNotHaveBean(ScheduledAnnotationBeanPostProcessor.class)
                    .doesNotHaveBean("org.springframework.context.annotation.internalScheduledAnnotationProcessor"));
  }
}
