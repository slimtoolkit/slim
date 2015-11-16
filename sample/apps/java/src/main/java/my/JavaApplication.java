package my;

import io.dropwizard.Application;
import io.dropwizard.setup.Bootstrap;
import io.dropwizard.setup.Environment;
import my.core.Template;
import my.resources.AppResource;
import my.health.TemplateHealthCheck;


public class JavaApplication extends Application<AppConfiguration> {
    public static void main(String[] args) throws Exception {
        new JavaApplication().run(args);
    }

    @Override
    public String getName() {
        return "java-app";
    }

    @Override
    public void initialize(Bootstrap<AppConfiguration> bootstrap) {
    }

    @Override
    public void run(AppConfiguration configuration, Environment environment) {
        final Template template = configuration.buildTemplate();

        environment.healthChecks().register("template", new TemplateHealthCheck(template));
        environment.jersey().register(new AppResource(template));
    }
}
