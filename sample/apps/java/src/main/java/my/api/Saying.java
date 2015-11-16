package my.api;

import com.fasterxml.jackson.annotation.JsonProperty;
import com.google.common.base.MoreObjects;
import org.hibernate.validator.constraints.Length;

public class Saying {
    final private String status = "success";
    final private String service = "java";
    private long id;

    @Length(max = 3)
    private String info;

    public Saying() {
        // Jackson deserialization
    }

    public Saying(long id, String info) {
        this.id = id;
        this.info = info;
    }

    @JsonProperty
    public long getId() {
        return id;
    }

    @JsonProperty
    public String getInfo() {
        return info;
    }

    @JsonProperty
    public String getStatus() {
        return status;
    }

    @JsonProperty
    public String getService() {
        return service;
    }

    @Override
    public String toString() {
        return MoreObjects.toStringHelper(this)
                .add("id", id)
                .add("info", info)
                .add("service", info)
                .add("status", info)
                .toString();
    }
}
