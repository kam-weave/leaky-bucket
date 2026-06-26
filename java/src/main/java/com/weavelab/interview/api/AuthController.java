package com.weavelab.interview.api;

import com.weavelab.interview.auth.AuthFilter;
import jakarta.servlet.http.HttpServletRequest;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.Map;

@RestController
@RequestMapping("/api/auth")
public class AuthController {

    @PostMapping("/token")
    public ResponseEntity<Map<String, String>> token(HttpServletRequest request) {
        String userId = (String) request.getAttribute(AuthFilter.USER_ID_ATTRIBUTE);
        return ResponseEntity.ok(Map.of(
                "user_id", userId,
                "status", "authenticated"
        ));
    }
}
